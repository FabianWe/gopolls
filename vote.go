// Copyright 2020 Fabian Wenzelmann <fabianwen@posteo.eu>
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package gopolls

import (
	"encoding/csv"
	"fmt"
	"io"
)

type AbstractVote interface {
	GetVoter() *Voter
	VoteType() string
}

type VoteParser interface {
	ParseFromString(s string, voter *Voter) (AbstractVote, error)
}

const (
	BasicVoteType   = "basic-vote"
	MedianVoteType  = "median-vote"
	SchulzeVoteType = "schulze-vote"
)

// CSV //

type VotesCSVWriter struct {
	csv *csv.Writer
}

func NewVotesCSVWriter(w io.Writer) *VotesCSVWriter {
	writer := csv.NewWriter(w)
	writer.Comma = ';'
	return &VotesCSVWriter{csv: writer}
}

func (w *VotesCSVWriter) writeCSVHead(skels []AbstractPollSkeleton) error {
	row := make([]string, len(skels)+1)
	row[0] = "voter"
	for i, skel := range skels {
		row[i+1] = skel.GetName()
	}
	return w.csv.Write(row)
}

func (w *VotesCSVWriter) writeEmptyRecords(voters []*Voter, skels []AbstractPollSkeleton) error {
	// row will be re-used
	row := make([]string, len(skels)+1)
	for _, voter := range voters {
		row[0] = voter.Name
		for j, skel := range skels {
			row[j+1] = skel.GetName()
		}
		if err := w.csv.Write(row); err != nil {
			return err
		}
	}
	return nil
}

func (w *VotesCSVWriter) GenerateEmptyTemplate(voters []*Voter, skels []AbstractPollSkeleton) error {
	if err := w.writeCSVHead(skels); err != nil {
		return err
	}
	if err := w.writeEmptyRecords(voters, skels); err != nil {
		return err
	}
	w.csv.Flush()
	return w.csv.Error()
}

type VotesCSVReader struct {
	csv *csv.Reader
}

func NewVotesCSVReader(r io.Reader) *VotesCSVReader {
	reader := csv.NewReader(r)
	reader.Comma = ';'
	return &VotesCSVReader{
		csv: reader,
	}
}

func (r *VotesCSVReader) readHead() ([]string, error) {
	res, err := r.csv.Read()
	if err == io.EOF {
		return nil, NewPollingSyntaxError(nil, "no header found in csv file")
	}
	if err != nil {
		return nil, err
	}
	if len(res) == 0 {
		return nil, NewPollingSyntaxError(nil, "expected at least the voter column in csv file")
	}
	return res, nil
}

func (r *VotesCSVReader) ReadRecords() (head []string, lines [][]string, err error) {
	head, err = r.readHead()
	if err != nil {
		return
	}
	// note that the first call in read head already makes sure that each line has the exact
	// same length and that the length is > 0
	lines, err = r.csv.ReadAll()
	if err != nil {
		head = nil
		lines = nil
	}
	return
}

type VotersMatrix struct {
	Voters      []*Voter
	Polls       *PollSkeletonCollection
	MatrixHead  []string
	Matrix      [][]string
	VotersMap   map[string]*Voter
	SkeletonMap map[string]AbstractPollSkeleton
}

func NewVotersMatrixFromCSV(r io.Reader, voters []*Voter, polls *PollSkeletonCollection) (*VotersMatrix, error) {
	votersMap, votersMapErr := VotersToMap(voters)
	if votersMapErr != nil {
		return nil, votersMapErr
	}
	pollsMap, pollsMapErr := polls.SkeletonsToMap()
	if pollsMapErr != nil {
		return nil, pollsMapErr
	}
	csvReader := NewVotesCSVReader(r)
	// read head and body of matrix
	head, matrix, csvErr := csvReader.ReadRecords()
	if csvErr != nil {
		return nil, csvErr
	}
	res := &VotersMatrix{
		Voters:      voters,
		Polls:       polls,
		MatrixHead:  head,
		Matrix:      matrix,
		VotersMap:   votersMap,
		SkeletonMap: pollsMap,
	}
	return res, nil
}

func (m *VotersMatrix) PrepareAndVerifyVotesMatrix() ([]*Voter, []AbstractPollSkeleton, error) {
	if len(m.MatrixHead) == 0 {
		return nil,
			nil,
			NewPollingSyntaxError(nil, "votes matrix must contain at least one column (voter name)")
	}
	// some simple checks to avoid doing complicated stuff if we can already find discrepancies here
	if len(m.VotersMap) != len(m.Matrix) {
		return nil,
			nil,
			NewPollingSyntaxError(nil, "length of votersMap matrix does not match number of given votersMap")
	}
	if len(m.SkeletonMap) != len(m.MatrixHead)-1 {
		return nil,
			nil,
			NewPollingSyntaxError(nil, "length of polls matrix does not match number of given polls")
	}
	//now read the votersMap from the matrix and ensure that each voter in the matrix (first column)
	// is also in the original mapping
	// create a list of votersMap on the fly
	newVoters := make([]*Voter, 0, len(m.VotersMap))
	newVotersSet := make(map[string]struct{}, len(m.VotersMap))
	for _, row := range m.Matrix {
		if len(row) != len(m.MatrixHead) {
			return nil,
				nil,
				NewPollingSyntaxError(nil, "number of columns in votersMap matrix must always be %d, got row of length %d instead",
					len(m.MatrixHead), len(row))
		}
		voterName := row[0]
		if _, alreadyFound := newVotersSet[voterName]; alreadyFound {
			return nil,
				nil,
				NewDuplicateError(fmt.Sprintf("voter \"%s\" was found multiple times in the votersMap matrix", voterName))
		}
		voter, has := m.VotersMap[voterName]
		if !has {
			return nil,
				nil,
				NewPollingSyntaxError(nil, "voter \"%s\" from votersMap matrix not found in original list", voterName)
		}
		newVoters = append(newVoters, voter)
		newVotersSet[voterName] = struct{}{}
	}
	// now all votersMap exist and we created a list of them
	// also we took care that this list doesn't contain duplicates
	// if the lengths of the two voter sets are equal they must be identical
	if len(m.VotersMap) != len(newVotersSet) {
		return nil,
			nil,
			NewPollingSyntaxError(nil, "not all votersMap from source were found in the votersMap matrix")
	}
	// now do the same for all skeletons
	newSkeletons := make([]AbstractPollSkeleton, 0, len(m.SkeletonMap))
	newSkeletonsSet := make(map[string]struct{}, len(m.SkeletonMap))
	for _, pollName := range m.MatrixHead[1:] {
		if _, alreadyFound := newVotersSet[pollName]; alreadyFound {
			return nil,
				nil,
				NewDuplicateError(fmt.Sprintf("poll \"%s\" was found multiple times in the votersMap matrix", pollName))
		}
		skel, has := m.SkeletonMap[pollName]
		if !has {
			return nil,
				nil,
				NewPollingSyntaxError(nil, "poll \"%s\" from votersMap matrix not found in original list", pollName)
		}
		newSkeletons = append(newSkeletons, skel)
		newSkeletonsSet[pollName] = struct{}{}
	}
	// now again, all polls exist and we created a list of them, test the lengths as before
	if len(m.SkeletonMap) != len(newSkeletonsSet) {
		return nil,
			nil,
			NewPollingSyntaxError(nil, "not all polls from source were found in the votersMap matrix")
	}
	return newVoters, newSkeletons, nil
}
