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
	"errors"
	"io"
)

type AbstractVote interface {
	GetVoter() *Voter
	VoteType() string
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
		return nil, errors.New("no header found in csv file")
	}
	if err != nil {
		return nil, err
	}
	if len(res) == 0 {
		return nil, errors.New("expected at least the voter column in csv file")
	}
	return res, nil
}

func (r *VotesCSVReader) ReadRecord() (head []string, lines [][]string, err error) {
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
