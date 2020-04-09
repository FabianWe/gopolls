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
	"reflect"
)

// AbstractVote describes an abstract vote (usually each poll has one vote type).
//
// It can retrieve the voter that voted and a type string which must be unique for all implementations.
type AbstractVote interface {
	GetVoter() *Voter
	VoteType() string
}

// VoteParser parses a vote from a string.
//
// Returned errors should be an internal error type like PollingSyntaxError or PollingSemanticError.
type VoteParser interface {
	ParseFromString(s string, voter *Voter) (AbstractVote, error)
}

// ParserGenerationError is an error that is returned if no parser could be created for a skeleton.
//
// Usually a skeleton "implies" a parser, for example sets the number of expected options or a maximal value
// on a parser.
// This error is used to signal that no parser could be created for a skeleton / description.
type ParserGenerationError struct {
	PollError
	Msg string
}

// NewParserGenerationError returns a new ParserGenerationError.
//
// The error message can be formatted like in fmt.Sprintf().
func NewParserGenerationError(msg string, a ...interface{}) ParserGenerationError {
	return ParserGenerationError{
		Msg: fmt.Sprintf(msg, a...),
	}
}

func (err ParserGenerationError) Error() string {
	return err.Msg
}

// GenerateDefaultParsers creates the parser for a list of polls.
//
// Usually a skeleton "implies" a parser, for example sets the number of expected options or a maximal value
// on a parser.
// Thus each skeleton defines its own parser instance.
// This method creates such parsers.
//
// This is a method that is rather tailored for my use case, but it should give you a general idean
// how to parse votes.
//
// The templates (like basicPollTemplate) are the parsers that are used to create new parsers from.
// For example the NewMedianVoteParser has a method WithMaxValue. We use the provided parser template
// and call WithMaxValue for a MedianPoll.
//
// All parsers can be nil in which case they default to:
// basicPollTemplate ==> NewBasicVoteParser()
// medianPollTemplate ==> NewMedianVoteParser(DefaultCurrencyHandler)
// schulzePollTemplate ==> NewSchuleVoteParser(-1)
//
// By providing other defaults you can overwrite the parser behavior.
//
// It returns a ParserGenerationError if the poll is not supported (BasicPoll, MedianPoll or SchulzePoll)..
func GenerateDefaultParsers(polls []AbstractPoll,
	basicPollTemplate *BasicVoteParser,
	medianPollTemplate *MedianVoteParser,
	schulzePollTemplate *SchulzeVoteParser,
) ([]VoteParser, error) {
	// set templates to defaults if nil
	if basicPollTemplate == nil {
		basicPollTemplate = NewBasicVoteParser()
	}
	if medianPollTemplate == nil {
		medianPollTemplate = NewMedianVoteParser(DefaultCurrencyHandler)
	}
	if schulzePollTemplate == nil {
		schulzePollTemplate = NewSchuleVoteParser(-1)
	}
	res := make([]VoteParser, len(polls))

	for i, poll := range polls {
		var parser VoteParser
		switch typedPoll := poll.(type) {
		case *BasicPoll:
			parser = basicPollTemplate
		case *MedianPoll:
			parser = medianPollTemplate.WithMaxValue(typedPoll.Value)
		case *SchulzePoll:
			parser = schulzePollTemplate.WithLength(typedPoll.NumOptions)
		default:
			return nil, NewParserGenerationError("unsupported poll of type %s", reflect.TypeOf(poll))
		}
		res[i] = parser
	}

	return res, nil
}

const (
	BasicVoteType   = "basic-vote"
	MedianVoteType  = "median-vote"
	SchulzeVoteType = "schulze-vote"
)

// CSV //

const DefaultCSVSeparator = ','

// VotesCSVWriter can be used to create a CSV file template for inserting polls in it.
// Refer to the wiki for details about CSV files.
type VotesCSVWriter struct {
	Sep rune
	csv *csv.Writer
}

// NewVotesCSVWriter returns a new VotesCSVWriter writing to w.
func NewVotesCSVWriter(w io.Writer) *VotesCSVWriter {
	writer := csv.NewWriter(w)
	return &VotesCSVWriter{
		Sep: DefaultCSVSeparator,
		csv: writer,
	}
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
		if err := w.csv.Write(row); err != nil {
			return err
		}
	}
	return nil
}

// GenerateEmptyTemplate generates an empty CSV template (contains all polls and voters, but no votes).
//
// It returns any errors from writing to w.
func (w *VotesCSVWriter) GenerateEmptyTemplate(voters []*Voter, skels []AbstractPollSkeleton) error {
	w.csv.Comma = w.Sep
	if err := w.writeCSVHead(skels); err != nil {
		return err
	}
	if err := w.writeEmptyRecords(voters, skels); err != nil {
		return err
	}
	w.csv.Flush()
	return w.csv.Error()
}

// VotesCSVReader can be used to parse a CSV file of votes (see wiki for details about CSV files).
// It can only be used to parse the "matrix", that is the strings from the CSV file.
// No conversion to a vote object is done, it reads the pure strings which then need to be processed further.
// For an example see NewVotersMatrixFromCSV, but you probably want your own method for dealing with parsed CSV
// files.
type VotesCSVReader struct {
	Sep rune
	csv *csv.Reader
}

// wrapError wraps an error that occurred during reading, if it is a CSV parse error it returns a PollingSyntaxError.
// The CSV error is not wrapped so clients don't rely on the csv internal errors.
// It must only be called with err != nil.
func (r *VotesCSVReader) wrapError(err error) error {
	if asCsvErr, ok := err.(*csv.ParseError); ok {
		return NewPollingSyntaxError(nil, asCsvErr.Error())
	}
	return err
}

// NewVotesCSVReader returns a VotesCSVReader reading from r.
func NewVotesCSVReader(r io.Reader) *VotesCSVReader {
	reader := csv.NewReader(r)
	return &VotesCSVReader{
		Sep: DefaultCSVSeparator,
		csv: reader,
	}
}

func (r *VotesCSVReader) readHead() ([]string, error) {
	res, err := r.csv.Read()
	if err == io.EOF {
		return nil, NewPollingSyntaxError(nil, "no header found in csv file")
	}
	if err != nil {
		return nil, r.wrapError(err)
	}
	if len(res) == 0 {
		return nil, NewPollingSyntaxError(nil, "expected at least the voter column in csv file")
	}
	return res, nil
}

// ReadRecords reads the records from the CSV file.
//
// The head should always be of the form
// ["Voter", <poll_name1>, <poll_name2>, ...., <poll_nameN>].
//
// The body (the lines part) should always be of the form
// [<voter_name>, <vote_for_poll1>, <vote_for_poll2>, ..., <vote_for_pollN>].
//
// It returns any error reading from the source.
// It might also return a PollingSyntaxError if the file is not correctly formed.
func (r *VotesCSVReader) ReadRecords() (head []string, lines [][]string, err error) {
	r.csv.Comma = r.Sep
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
		err = r.wrapError(err)
	}

	return
}

// VotersMatrix describes a matrix has it has been parsed from a csv file.
//
// VotesCSVReader gives you a method to read the lines and return it as a "string" matrix.
// This struct can be used to represent additional information about the matrix, for example see
// NewVotersMatrixFromCSV.
// But this method (and this whole type) is rather tailored to my use case, but it should give you an idea.
//
// Voters and Polls are not the entries parsed from the matrix! They're pre-existing instances with which
// the csv file gets matched later on.
type VotersMatrix struct {
	Voters      []*Voter
	Polls       *PollSkeletonCollection
	MatrixHead  []string
	Matrix      [][]string
	VotersMap   map[string]*Voter
	SkeletonMap map[string]AbstractPollSkeleton
}

// NewVotersMatrixFromCSV reads a CSV file from r and transforms it into a VotersMatrix.
//
// It will create all both maps from voters and polls (by calling VotersToMap and PollSkeletonCollection.SkeletonsToMap)
// and return any errors from there.
// It will however not match the voters parsed from the CSV file against the voters list provided, for example
// make sure that each voter exists, no in the csv file is not contained within original list, no voter in the
// csv file appears multiple times etc.
//
// PrepareAndVerifyVotesMatrix will do this for you however.
func NewVotersMatrixFromCSV(r *VotesCSVReader, voters []*Voter, polls *PollSkeletonCollection) (*VotersMatrix, error) {
	votersMap, votersMapErr := VotersToMap(voters)
	if votersMapErr != nil {
		return nil, votersMapErr
	}
	pollsMap, pollsMapErr := polls.SkeletonsToMap()
	if pollsMapErr != nil {
		return nil, pollsMapErr
	}
	// read head and body of matrix
	head, matrix, csvErr := r.ReadRecords()
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

// PrepareAndVerifyVotesMatrix will ensure that: The matrix is not empty, that no duplicates exist in the
// source (voter name, poll name).
// It also makes sure that each voter in the csv source also exists in the given source and that no voter is
// missing (and the same for polls).
// It returns the list of voters and polls in the order in which they appeared in the csv file.
// The length of these results must then be equal to the length of the provided voters and skeletons.
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
			NewPollingSyntaxError(nil, "length of votersMap matrix does not match number of given voters")
	}
	if len(m.SkeletonMap) != len(m.MatrixHead)-1 {
		return nil,
			nil,
			NewPollingSyntaxError(nil, "length of polls matrix does not match number of given polls")
	}
	// now read the votersMap from the matrix and ensure that each voter in the matrix (first column)
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

type EmptyVotePolicy int8

const (
	IgnoreEmpty EmptyVotePolicy = iota
	AddAsNoEmpty
	AddAsAbstention
)

func (m *VotersMatrix) generateVotesForPoll(pollIndex int, poll AbstractPoll, parser VoteParser) error {
	// iterate over all voters and try to parse vote
	return nil
}

func (m *VotersMatrix) DefaultFillWithVotes(pollsList []AbstractPoll, parsers []VoteParser) error {
	// TODO use new error type?
	if len(pollsList) != len(parsers) {
		return fmt.Errorf("can't generate votes, expected %d parsers (one for each poll), but got %d parsers",
			len(pollsList), len(parsers))
	}
	if numPolls := m.Polls.NumSkeletons(); numPolls != len(parsers) {
		return fmt.Errorf("can't generate votes, expected %d parsers (one for each poll), but got %d parsers",
			numPolls, len(parsers))
	}

	return nil
}
