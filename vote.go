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
	"strings"
)

// AbstractVote describes an abstract vote (usually each poll has one vote type).
//
// It can retrieve the voter that voted and a type string which must be unique for all implementations.
type AbstractVote interface {
	GetVoter() *Voter
	VoteType() string
}

const (
	BasicVoteType   = "basic-vote"
	MedianVoteType  = "median-vote"
	SchulzeVoteType = "schulze-vote"
)

// VoteParser parses a vote from a string.
//
// Returned errors should be an internal error type like PollingSyntaxError or PollingSemanticError.
// If the error is not nil the returned vote is not allowed to be nil
//
// It is recommended to also implement ParserCustomizer.
type VoteParser interface {
	ParseFromString(s string, voter *Voter) (AbstractVote, error)
}

// ParserCustomizer is a parser that allows customization based on a poll.
//
// For example a median poll can be customized by setting a max value, that is the maximal value this parser will parse.
// The workflow then is this: Create a parser "template" with the default options you want to use and then customize
// it for each poll with CustomizeForPoll.
// An example and a helper function is given CustomizeParsers.
//
// In the median example: The template consists of a parser that allows all valid numbers / integers.
// It is then customized for a certain poll by setting the max value of that poll.
//
// The CustomizeForPoll method should return the customized parser, if poll is of the wrong type or the operation is
// in some way not allowed a PollTypeError should be returned.
//
// All parsers from this package also implement this interface.
type ParserCustomizer interface {
	VoteParser
	CustomizeForPoll(poll AbstractPoll) (ParserCustomizer, error)
}

// DefaultParserTemplateMap contains default templates for BasicPollType, MedianPollType and SchulzePollType.
// Of course it could be extended by other init functions.
var DefaultParserTemplateMap = make(map[string]ParserCustomizer, 3)

// init adds default templates
func init() {
	DefaultParserTemplateMap[BasicPollType] = NewBasicVoteParser()
	DefaultParserTemplateMap[MedianPollType] = NewMedianVoteParser(DefaultCurrencyHandler)
	DefaultParserTemplateMap[SchulzePollType] = NewSchulzeVoteParser(-1)
}

// CustomizeParsers customizes all polls with a given template.
//
// As discussed in the documentation for ParserCustomizer each parser can be customized for a specific poll.
// This method will apply CustomizeForPoll on a list of polls.
//
// The templates map must have an entry for each poll type string.
// For example a BasicPoll returns BasicPollType in PollType(). This string must be mapped to a ParserCustomizer
// that works as the template for all BasicPolls.
//
// DefaultParserTemplateMap contains some default templates for BasicPollType, MedianPollType and SchulzePollType.
//
// Of course there are other ways to do the conversion, but this is a nice helper function.
//
// It returns a PollTypeError if a template is not found in templates and returns any error from calls to
// CustomizeForPoll.
func CustomizeParsers(polls []AbstractPoll, templates map[string]ParserCustomizer) ([]ParserCustomizer, error) {
	res := make([]ParserCustomizer, len(polls))
	for i, poll := range polls {
		// get the parserTemplate
		parserTemplate, hasTemplate := templates[poll.PollType()]
		if !hasTemplate {
			return nil,
				NewPollTypeError("no matching parser parserTemplate for type %s (name %s) found",
					reflect.TypeOf(poll), poll.PollType())
		}
		// try to customize
		customized, customizeErr := parserTemplate.CustomizeForPoll(poll)
		if customizeErr != nil {
			return nil, customizeErr
		}
		res[i] = customized
	}
	return res, nil
}

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
// The CSV error is not wrapped so clients don't rely on the csv internal errors, only the string is copied.
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

// VotersMatrix describes a matrix as it has been parsed from a csv file.
//
// VotesCSVReader gives you a method to read the lines and return it as a "string" matrix.
// This struct can be used to represent additional information about the matrix, for example see
// NewVotersMatrixFromCSV.
// But this method (and this whole type) is rather tailored to my use case, but it should give you an idea.
//
// The Voters and Poll fields represent the order of the votes in the matrix, thus this exact order can be
// used.
//
// VerifyAndFill checks that the string matrix has the correct form (each voter has an entry for each poll)
// and that each voter and poll exist.
// That is it expects that the maps in the matrix are set (from some existing source for example a database).
// Then it verifies that the string matrix is of the correct form (each voter has one entry for each poll).
// It also verifies that each voter, that only has a name in the string matrix, exists in the VotersMap.
// The same verification is done for polls: Each poll in the string matrix (given by name) must exist in
// SkeletonMap.
// It will then set the Voters and Polls slices.
//
// Thus the workflow is as follows:
// Create a matrix given a collection of voters and of polls, that is set VotersMap and SkeletonMap and
// also set the string entries of the matrix (MatrixHead and MatrixBody).
// Then call VerifyAndFill to verify the input and set the Voters and Polls slices in the matrix.
//
// All other methods usually assume that the matrix is correct and the described properties hold.
//
// See NewVotersMatrixFromCSV for an example.
type VotersMatrix struct {
	Voters      []*Voter
	Polls       []AbstractPollSkeleton
	VotersMap   map[string]*Voter
	SkeletonMap map[string]AbstractPollSkeleton

	MatrixHead []string
	MatrixBody [][]string
}

// NewVotersMatrixFromCSV reads the records from a CSV file (see wiki for format).
//
// It also verifies the matrix that was read by calling VerifyAndFill, this also sets the
// Voters and Polls slices.
// See type documentation for details.
func NewVotersMatrixFromCSV(r *VotesCSVReader, voters []*Voter, polls *PollSkeletonCollection) (*VotersMatrix, error) {
	// read csv, then create actual content
	head, body, csvErr := r.ReadRecords()
	if csvErr != nil {
		return nil, csvErr
	}
	// create mappings, they can be created from voters and polls
	// we find errors with VerifyAndFill later, thus each voter / poll must exist if VerifyAndFill returns no
	// error

	votersMap, votersMapErr := VotersToMap(voters)
	if votersMapErr != nil {
		return nil, votersMapErr
	}
	pollsMap, pollsMapErr := polls.SkeletonsToMap()
	if pollsMapErr != nil {
		return nil, pollsMapErr
	}

	res := &VotersMatrix{
		Voters:      nil,
		Polls:       nil,
		VotersMap:   votersMap,
		SkeletonMap: pollsMap,
		MatrixHead:  head,
		MatrixBody:  body,
	}
	if validateErr := res.VerifyAndFill(); validateErr != nil {
		return nil, validateErr
	}
	return res, nil
}

// VerifyAndFill tests that the string matrix (MatrixHead and MatrixBody) are well formed.
//
// It assumes that VotersMap, SkeletonMap, MatrixHead and MatrixBody are set.
// It will then make sure that each voter (from VotersMap) appears exactly once and has a vote for each poll.
// It also checks that each poll (from SkeletonMap) appears exactly once.
//
// I.e. each voter / poll from the source maps must be found in some row / column and in the string matrix
// it must appear only once.
//
// NewVotersMatrixFromCSV already calls this function.
func (m *VotersMatrix) VerifyAndFill() error {
	numVoters := len(m.VotersMap)
	numPolls := len(m.SkeletonMap)
	if len(m.MatrixHead) == 0 {
		return NewPollingSemanticError(nil, "votes matrix must contain at least one column (voter name)")
	}
	// some simple checks to avoid doing complicated stuff if we can already find discrepancies here
	if numVoters != len(m.MatrixBody) {
		return NewPollingSemanticError(nil,
			"invalid voters matrix length: expected one entry for each of the %d voters, matrix contains %d rows",
			numVoters, len(m.MatrixBody))
	}
	if numPolls != len(m.MatrixHead)-1 {
		return NewPollingSemanticError(nil,
			"invalid voters matrix: head must contain voter column and exactly one entry for each poll, number of polls is %d, head contains %d entries",
			numPolls, len(m.MatrixHead))
	}

	// verify voters
	if votersVerificationErr := m.verifyAndFillVoters(); votersVerificationErr != nil {
		return votersVerificationErr
	}

	// verify polls
	if pollsVerificationErr := m.verifyAndFillPolls(); pollsVerificationErr != nil {
		return pollsVerificationErr
	}

	return nil
}

func (m *VotersMatrix) verifyAndFillVoters() error {
	numVoters := len(m.VotersMap)
	// now ensure that each voter from the matrix is also contained in the original voters list
	// create the list of voters on the fly
	m.Voters = make([]*Voter, 0, numVoters)
	votersSet := make(map[string]struct{}, numVoters)

	for _, row := range m.MatrixBody {
		if len(row) != len(m.MatrixHead) {
			return NewPollingSyntaxError(nil, "number of columns in votersMap matrix body must always be %d, got row of length %d instead",
				len(m.MatrixHead), len(row))
		}
		// len(head) >= 0 from check above
		voterName := row[0]
		// check if already found
		if _, alreadyFound := votersSet[voterName]; alreadyFound {
			return NewDuplicateError(fmt.Sprintf("voter \"%s\" was found multiple times in the matrix body",
				voterName))
		}
		// make sure the original one exists
		voter, has := m.VotersMap[voterName]
		if !has {
			return NewPollingSemanticError(nil, "voter \"%s\" from matrix not found in allowed voters",
				voterName)
		}
		// is unique and exists, add voter
		m.Voters = append(m.Voters, voter)
		votersSet[voterName] = struct{}{}
	}
	// now all voters exist and are unique
	// now assert that each voter from the original list also exists
	// if the lengths of the two sets are equal the sets must be equal
	if numVoters != len(votersSet) {
		return NewPollingSemanticError(nil, "not all voters from source exist in matrix")
	}
	return nil
}

func (m *VotersMatrix) verifyAndFillPolls() error {
	numPolls := len(m.SkeletonMap)
	m.Polls = make([]AbstractPollSkeleton, 0, numPolls)
	pollsSet := make(map[string]struct{}, numPolls)

	for _, pollName := range m.MatrixHead[1:] {
		// make sure name is unique
		if _, alreadyFound := pollsSet[pollName]; alreadyFound {
			return NewDuplicateError(fmt.Sprintf("poll \"%s\" was found multiple times in the matrix head",
				pollName))
		}

		// make sure the original one exists
		poll, has := m.SkeletonMap[pollName]
		if !has {
			return NewPollingSemanticError(nil, "poll \"%s\" from matrix not found in allowed polls",
				pollName)
		}
		// is unique and exists, add it
		m.Polls = append(m.Polls, poll)
		pollsSet[pollName] = struct{}{}
	}
	// now all voters exist and are unique
	// now assert that each voter from the original list also exists
	// if the lengths of the two sets are equal the sets must be equal
	if numPolls != len(pollsSet) {
		return NewPollingSemanticError(nil, "not all polls from source exist in matrix")
	}
	return nil
}

// VoteGenerator is used to describe polls that can produce a poll specific vote type for a basic answer
// (yes, no or abstention).
//
// It is not allowed to return a nil vote and error = nil, that is if there is no error the returned
// vote is not allowed to be nil.
//
// It should return a PollTypeError if an answer is not supported (or not at all).
// All polls implemented at the moment implement this interface.
type VoteGenerator interface {
	AbstractPoll
	GenerateVoteFromBasicAnswer(voter *Voter, answer BasicPollAnswer) (AbstractVote, error)
}

// EmptyVotePolicy describes the behavior if an "empty" vote is found.
//
// By empty vote we mean that a certain voter just didn't cast a vote for a poll.
// If this is the case there are different things to do, depending on the application.
//
// The option that is chosen most often will probably be IgnoreEmptyVote: For a voter
// there is no vote so just assume that the voter wasn't there when the poll took place
// and ignore it (IgnoreEmptyVote).
//
// It is also possible to have polls where each voter has to cast a vote, in this case an
// error should be returned (RaiseErrorEmptyVote).
//
// In certain cases even voters who didn't cast a vote should be considered, for example in
// absolute majorities.
// The empty votes can then be treated as "No", "Aye" or "Abstention" (No and Abstention or the most
// likely options here). These are described by the policies AddAsAyeEmptyVote, AddAsNoEmptyVote
// and AddAsAbstentionEmptyVote.
type EmptyVotePolicy int8

const (
	IgnoreEmptyVote EmptyVotePolicy = iota
	RaiseErrorEmptyVote
	AddAsAyeEmptyVote
	AddAsNoEmptyVote
	AddAsAbstentionEmptyVote
)

// GeneratePoliciesList is just a small helper function that returns a list of num elements, each entry is
// set to the given policy.
func GeneratePoliciesList(policy EmptyVotePolicy, num int) []EmptyVotePolicy {
	res := make([]EmptyVotePolicy, num)
	for i := 0; i < num; i++ {
		res[i] = policy
	}
	return res
}

// ErrEmptyPollPolicy is an error used if a policy is set to RaiseErrorEmptyVote and an empty vote was found.
// GenerateEmptyVoteForVoter will in this case return an error e s.t. errors.Is(e, ErrEmptyPollPolicy) returns true.
// This should of course be checked before errors.Is(e, ErrPoll) because this is true for all internal errors.
var ErrEmptyPollPolicy = NewPollTypeError("empty votes are not allowed")

// GenerateEmptyVoteForVoter can be called to generate a vote for a poll if the input was empty.
// By empty we mean that the voter simple didn't cast a vote.
// If this method is called it will chose the action depending on the policy.

// If it is IgnoreEmptyVote it returns nil for the vote and nil as an error.
// So be aware that a vote can be nil even if the error is nil.
//
// If the policy is RaiseErrorEmptyVote an error will be returned.
//
// In all other cases poll must implement VoteGenerator (if it does not an error is returned)
// and the return value depends on the poll which is responsible to create an Aye, No or
// Abstention vote.
//
// For the implemented types note that MedianPoll does not support abstention.
func (policy EmptyVotePolicy) GenerateEmptyVoteForVoter(voter *Voter, poll AbstractPoll) (AbstractVote, error) {
	switch policy {
	case IgnoreEmptyVote:
		return nil, nil
	case RaiseErrorEmptyVote:
		return nil, fmt.Errorf("voter \"%s\" and poll type \"%s\": %w",
			voter.Name, reflect.TypeOf(poll), ErrEmptyPollPolicy)
	}
	// in all other cases it must be called with a VoteGenerator
	// must be called with a VoteGenerator
	asGenerator, ok := poll.(VoteGenerator)
	if !ok {
		return nil, NewPollTypeError("can only generate a poll for polls that implement VoteGenerator, got type %s",
			reflect.TypeOf(poll))
	}

	switch policy {
	case AddAsAyeEmptyVote:
		return asGenerator.GenerateVoteFromBasicAnswer(voter, Aye)
	case AddAsNoEmptyVote:
		return asGenerator.GenerateVoteFromBasicAnswer(voter, No)
	case AddAsAbstentionEmptyVote:
		return asGenerator.GenerateVoteFromBasicAnswer(voter, Abstention)
	default:
		return nil, NewPollTypeError("invalid policy %d, can't generate vote for this policy",
			policy)
	}
}

func (m *VotersMatrix) generateSingleVote(poll AbstractPoll, parser VoteParser, policy EmptyVotePolicy, voter *Voter, s string) (AbstractVote, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return policy.GenerateEmptyVoteForVoter(voter, poll)
	}
	// not empty, so take the parser
	return parser.ParseFromString(s, voter)
}

func (m *VotersMatrix) generateVotesForPoll(columnIndex int, poll AbstractPoll, parser VoteParser, policy EmptyVotePolicy) error {
	// iterate over all voters and generate the vote
	// this could be nil due to the policy, in which case it should be ignored
	for i, voter := range m.Voters {
		voteString := m.MatrixBody[i][columnIndex]
		vote, voteErr := m.generateSingleVote(poll, parser, policy, voter, voteString)
		if voteErr != nil {
			return voteErr
		}
		if vote != nil {
			// add to poll
			if addErr := poll.AddVote(vote); addErr != nil {
				return addErr
			}
		}
	}
	return nil
}

func (m *VotersMatrix) fillAllPolls(polls []AbstractPoll, parsers []VoteParser, policies []EmptyVotePolicy) error {
	// internal struct used in a channel
	type pollParseRes struct {
		pollIndex int
		err       error
	}

	// channel for communication
	ch := make(chan pollParseRes, 1)

	// parse all votes for all polls concurrently with generateVotesForPoll
	for i, p := range polls {
		go func(index int, poll AbstractPoll) {
			// index + 1 because the first column always contains the voter names
			colErr := m.generateVotesForPoll(index+1, poll, parsers[index], policies[index])
			ch <- pollParseRes{
				pollIndex: index,
				err:       colErr,
			}
		}(i, p)
	}

	// we capture the error in the smallest column and return it
	var err error
	smallestPollIndex := -1

	for i := 0; i < len(polls); i++ {
		colRes := <-ch
		if colRes.err != nil && (smallestPollIndex < 0 || colRes.pollIndex < smallestPollIndex) {
			err = colRes.err
			smallestPollIndex = colRes.pollIndex
		}
	}

	return err
}

// idea: first convert skeleton to polls, then create parsers, then this method
// before: validate
func (m *VotersMatrix) FillVotesFromMatrix(polls []AbstractPoll, parsers []VoteParser, policies []EmptyVotePolicy) error {
	numPolls := len(m.Polls)

	if numPolls != len(polls) {
		return NewPollingSemanticError(nil,
			"can't generate votes, expected %d polls (one for each skeleton), bug got %d polls instead",
			numPolls, len(polls))
	}

	if numPolls != len(parsers) {
		return NewPollingSemanticError(nil,
			"can't generate votes, expected %d parsers (one for each poll), but got %d parsers instead",
			numPolls, len(parsers))
	}

	if numPolls != len(policies) {
		return NewPollingSemanticError(nil,
			"can't generate votes, expected %d policies (one for each poll) but got %d policies instead",
			numPolls, len(policies))
	}

	// now insert
	return m.fillAllPolls(polls, parsers, policies)
}
