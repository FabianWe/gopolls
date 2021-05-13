// Copyright 2020, 2021 Fabian Wenzelmann <fabianwen@posteo.eu>
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
	"unicode/utf8"
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
// If the error is not nil the returned vote is not allowed to be nil.
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
// An example and a helper function is given in CustomizeParsers.
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
// Of course it can be extended.
// The easiest way to extend the default parsers is use to either insert values directly here or, if you don't want
// that, generate a fresh map with GenerateDefaultParserTemplateMap.
var DefaultParserTemplateMap = GenerateDefaultParserTemplateMap()

func GenerateDefaultParserTemplateMap() map[string]ParserCustomizer {
	res := make(map[string]ParserCustomizer, 3)
	res[BasicPollType] = NewBasicVoteParser()
	res[MedianPollType] = NewMedianVoteParser(DefaultCurrencyHandler)
	res[SchulzePollType] = NewSchulzeVoteParser(-1)
	return res
}

// CustomizeParsers customizes parser templates for each poll.
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
//
// CustomizeParsersToMap is a function that has the same functionality but for maps.
func CustomizeParsers(polls []AbstractPoll, templates map[string]ParserCustomizer) ([]ParserCustomizer, error) {
	res := make([]ParserCustomizer, len(polls))
	for i, poll := range polls {
		// get the parserTemplate
		parserTemplate, hasTemplate := templates[poll.PollType()]
		if !hasTemplate {
			return nil,
				NewPollTypeError("no matching parserTemplate for type %s (name %s) found",
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

// CustomizeParsersToMap customizes parser templates for each poll.
//
// For details see CustomizeParsers.
// This function will return one entry in the result map for each poll in polls.
func CustomizeParsersToMap(polls PollMap, templates map[string]ParserCustomizer) (map[string]ParserCustomizer, error) {
	res := make(map[string]ParserCustomizer, len(polls))
	for name, poll := range polls {
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
		res[name] = customized
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
// For an example see ReadMatrixFromCSV, but you probably want your own method for dealing with a parsed CSV
// file.
//
// Furthermore the parser can be configured to read only a certain amount of lines / put restrictions on the records read.
// This is the same idea as in VotersParser, see there for details of when you would want to use restrictions.
//
// The reader returned by NewVotesCSVReader sets all these validation fields to -1, meaning no restrictions.
//
// The following restrictions can be configured:
// MaxNumLines is the number of lines that are allowed in a polls file (including head). Therefor it must be a number >= 1.
// MaxRecordLength is th maximal length in bytes (not runes) a record in a row is allowed to have.
// MaxVotersNameLength is the maximal length a voter name is allowed to have.
// MaxPollNameLength is the maximal length a poll name is allowed to have.
type VotesCSVReader struct {
	Sep                 rune
	csv                 *csv.Reader
	MaxNumLines         int
	MaxVotersNameLength int
	MaxPollNameLength   int
	MaxRecordLength     int
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
		Sep:                 DefaultCSVSeparator,
		csv:                 reader,
		MaxNumLines:         -1,
		MaxVotersNameLength: -1,
		MaxPollNameLength:   -1,
		MaxRecordLength:     -1,
	}
}

func (r *VotesCSVReader) validateRow(row []string) error {
	for _, entry := range row {
		if !utf8.ValidString(entry) {
			return ErrInvalidEncoding
		}
		if r.MaxRecordLength >= 0 && len(entry) > r.MaxRecordLength {
			return NewParserValidationError(fmt.Sprintf("entry in csv is too long: got length %d, allowed max length is %d",
				len(entry), r.MaxRecordLength))
		}
	}
	return nil
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
	if validateErr := r.validateRow(res); validateErr != nil {
		return nil, validateErr
	}
	// all poll names must be valid too
	if r.MaxPollNameLength >= 0 {
		for _, pollName := range res[1:] {
			if len(pollName) > r.MaxPollNameLength {
				return nil, NewParserValidationError(fmt.Sprintf("poll name is too long: got length %d, allowed max length is %d",
					len(pollName), r.MaxPollNameLength))
			}
		}
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
	// this function only makes sure to return nil, nil if err != nil
	defer func() {
		if err != nil {
			head = nil
			lines = nil
		}
	}()
	r.csv.Comma = r.Sep
	head, err = r.readHead()
	if err != nil {
		return
	}
	// note that the first call in read head already makes sure that each line has the exact
	// same length and that the length is > 0

	// for validation we don't use ReadAll but iterate "by hand"
	lines = make([][]string, 0, defaultVotesSize)
	// set to 1 because head has been read already
	lineNum := 1
	maxNumLines := r.MaxNumLines
	// 0 doesn't make sense, we set it to 1
	if maxNumLines == 0 {
		maxNumLines = 1
	}
	for {
		lineNum++
		// again one here because of head, 0 wouldn't make sense
		if maxNumLines >= 0 && lineNum > maxNumLines {
			err = NewParserValidationError(fmt.Sprintf("there are too many lines: only %d lines in csv file are allowed", r.MaxNumLines))
			return
		}
		record, recordErr := r.csv.Read()
		if recordErr == io.EOF {
			return
		}
		if recordErr != nil {
			err = r.wrapError(recordErr)
			return
		}

		if validateRecordErr := r.validateRow(record); validateRecordErr != nil {
			err = validateRecordErr
			return
		}

		// now we must also validate the voter
		if voterName := record[0]; r.MaxVotersNameLength >= 0 && len(voterName) > r.MaxVotersNameLength {
			err = NewParserValidationError(fmt.Sprintf("voter name is too long: got length %d, allowed max length is %d",
				len(voterName), r.MaxVotersNameLength))
			return
		}

		// everything fine, append
		lines = append(lines, record)
	}
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
// GeneratePoliciesMap does the same for a map.
func GeneratePoliciesList(policy EmptyVotePolicy, num int) []EmptyVotePolicy {
	res := make([]EmptyVotePolicy, num)
	for i := 0; i < num; i++ {
		res[i] = policy
	}
	return res
}

// PolicyMap defines a mapping from poll name to an empty vote policy.
type PolicyMap map[string]EmptyVotePolicy

// GeneratePoliciesMap is just a small helper function that returns a PolicyMap for all polls in the given map.
// The map returned maps each poll name to the given policy.
func GeneratePoliciesMap(policy EmptyVotePolicy, polls PollMap) PolicyMap {
	res := make(PolicyMap, len(polls))
	for name := range polls {
		res[name] = policy
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
//
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

// PollMatrix describes the contents of data as from a VotesCSVReader.
//
// The implementation might be more specific to fit my needs, but I think it could be re-used by other projects and
// in anyway demonstrates how the library is constructed and all methods can be combined.
//
// In general the csv file only contains names for voters and polls, these usually should be matched against an
// existing collection of voters / polls.
//
// This type gives you methods to help to deal with this content:
// ReadMatrixFromCSV just creates the matrix from a reader.
type PollMatrix struct {
	Head []string
	Body [][]string
}

// ReadMatrixFromCSV creates a matrix and reads the content from the csv reader.
func ReadMatrixFromCSV(r *VotesCSVReader) (*PollMatrix, error) {
	head, body, err := r.ReadRecords()

	if err != nil {
		return nil, err
	}
	m := PollMatrix{
		Head: head,
		Body: body,
	}
	return &m, nil
}

// MatchEntries tests if the matrix is well-formed.
//
// The maps voters and polls are maps that specify the allowed names / voter names.
//
// By we-formed we mean: The matrix must have at least one column, it is contains the voters in the first row and
// each row describes a poll, thus each line in the body is of the form [voter, poll1, ..., pollN].
// If the matrix does not have this form a PollingSemanticError is returned.
//
// Each voter in any of the rows must be contained in the given voters map, otherwise PollingSemanticError error is
// returned.
// Each poll (given in the head columns [1:]) must exist in the polls map, otherwise a PollingSemanticError is returned.
// Also there are no duplicates allowed in the voter names and column names, if a duplicate is found a DuplicateError
// is returned.
//
// Note that it is allowed that a voter is missing, i.e. not contained in the CSV. The same is true for polls, not each
// poll must be contained.
// That's why this method returns two maps, they contain the voters and polls that were actually found.
// If you want to make sure that each voter / poll is contained in the csv just compare the lengths of the original
// map and the matched map.
//
// This function will do no parsing, i.e. creating actual votes from the entries in the csv. You can use
// FillPollsWithVotes for that.
func (m *PollMatrix) MatchEntries(voters VoterMap, polls PollMap) (matchedVoters VoterMap, matchedPolls PollMap, err error) {
	matchedVoters = make(VoterMap, len(voters))
	matchedPolls = make(PollMap, len(polls))

	// this function will just make sure to return nil maps if err is != nil
	defer func() {
		if err != nil {
			matchedVoters = nil
			matchedPolls = nil
		}
	}()

	if len(m.Head) == 0 {
		err = NewPollingSyntaxError(nil, "poll matrix must contain at least one column (voter name)")
		return
	}

	// now see if all voters exist and the names from csv are uniqe
	for _, row := range m.Body {
		if len(row) != len(m.Head) {
			err = NewPollingSyntaxError(nil, "number of columns in csv is invalid, expected length of %d (head), got length %d instead",
				len(m.Head), len(row))
			return
		}
		// len(head) >= 0 from check above
		voterName := row[0]
		// check if we have a duplicate
		if _, alreadyFound := matchedVoters[voterName]; alreadyFound {
			err = NewDuplicateError(fmt.Sprintf("voter \"%s\" was found multiple times in the matrix body",
				voterName))
			return
		}
		// make sure that the voter is valid, i.e. exists in the original map
		if voter, exists := voters[voterName]; exists {
			matchedVoters[voterName] = voter
		} else {
			err = NewPollingSemanticError(nil, "voter \"%s\" from matrix not found in allowed voters",
				voterName)
			return
		}
	}

	// the same for polls
	// m.Head[0] is the voter name column
	for _, pollName := range m.Head[1:] {
		if _, alreadyFound := matchedPolls[pollName]; alreadyFound {
			err = NewDuplicateError(fmt.Sprintf("poll \"%s\" was found multiple times in the matrix head",
				pollName))
			return
		}
		// make sure that the poll is valid, i.e. exists in the original map
		if poll, exists := polls[pollName]; exists {
			matchedPolls[pollName] = poll
		} else {
			err = NewPollingSemanticError(nil, "poll \"%s\" from matrix not found in allowed polls",
				pollName)
			return
		}
	}

	return
}

func (m *PollMatrix) generateSingleVote(poll AbstractPoll, parser VoteParser, policy EmptyVotePolicy, voter *Voter, s string) (AbstractVote, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return policy.GenerateEmptyVoteForVoter(voter, poll)
	}
	return parser.ParseFromString(s, voter)
}

func (m *PollMatrix) generateVotesForPoll(columnIndex int, voters VoterMap, poll AbstractPoll, parser VoteParser, policy EmptyVotePolicy) error {
	// iterate over all voters and generate the vote
	// this could be nil due to the policy, in which case it should be ignored
	for _, row := range m.Body {
		voterName := row[0]
		voter := voters[voterName]
		voteString := row[columnIndex]
		vote, voteErr := m.generateSingleVote(poll, parser, policy, voter, voteString)
		if voteErr != nil {
			return voteErr
		}
		// only if vote is not nil add it
		if vote != nil {
			if addErr := poll.AddVote(vote); addErr != nil {
				return addErr
			}
		}
	}
	return nil
}

func (m *PollMatrix) fillAllPolls(voters VoterMap, polls PollMap, parsers map[string]VoteParser, policies PolicyMap) error {
	// internal struct used in a channel
	type pollParseRes struct {
		column int
		name   string
		err    error
	}

	// channel for communication
	ch := make(chan pollParseRes, 1)

	// parse all votes for all polls (concurrently) with generateVotesForPoll
	for column, pollName := range m.Head[1:] {
		go func(column int, pollName string) {
			poll := polls[pollName]
			parser := parsers[pollName]
			policy := policies[pollName]
			// index + 1 because column starts with 0
			collErr := m.generateVotesForPoll(column+1, voters, poll, parser, policy)
			ch <- pollParseRes{
				column: column,
				name:   pollName,
				err:    collErr,
			}
		}(column, pollName)
	}

	// we capture the error in the smallest column and return it
	var err error
	smallestPollIndex := -1

	numPolls := len(m.Head) - 1

	for i := 0; i < numPolls; i++ {
		colRes := <-ch
		if colRes.err != nil && (smallestPollIndex < 0 || colRes.column < smallestPollIndex) {
			err = colRes.err
			smallestPollIndex = colRes.column

		}
	}
	return err
}

// FillPollsWithVotes does the actual parsing of votes, it creates new vote entries in the polls.
//
// It takes the map of existing polls, these polls get filled by calling AddVote on the poll object.
// Each poll must have a unique parser that is used to parse a vote for the poll, see ParserCustomizer of how this
// should be done. The parses map must map vote name to the parser instance.
// Each poll (also by name) must have an EmptyVotePolicy associated with it.
// If the csv entry is empty (string contains only whitespace) not the parse method of the parser is called but
// GenerateEmptyVoteForVoter for the policy associated with the poll.
//
// If a poll does not have parser / policy associated with it a PollingSemanticError is returned.
// Also all errors from AddVote are returned.
// The matrix is also verified with MatchEntries function and any error from this function is returned.
// The arguments allowMissingVoters and allowMissingPolls determine what should happen if a voter or poll is missing.
// As described in MatchEntries it is possible that some voters / polls do not appear in the csv.
// If allowMissingVoters is set to false a PollingSemanticError is returned if a voter is missing.
// Also if allowMissingPolls is set to false and poll is missing in the csv a PollingSemanticError is returned.
//
// If everything is okay this method returns nil as error and the actual voters / polls that appeared in the csv.
// Especially if allowMissingVoters = false and allowMissingPolls = false the result should return maps that are
// equivalent to the input maps.
//
// Note that if an error is returned it is possible that some of the polls got already filled with votes!
// In this case not all votes for a poll might be present and the whole operation should be marked as failure and
// probably none of the votes that already appear in some poll should be used.
func (m *PollMatrix) FillPollsWithVotes(polls PollMap, voters VoterMap,
	parsers map[string]VoteParser, policies PolicyMap,
	allowMissingVoters, allowMissingPolls bool) (actualVoters VoterMap, actualPolls PollMap, err error) {
	// first ensure matrix structure
	actualVoters, actualPolls, err = m.MatchEntries(voters, polls)
	if err != nil {
		return
	}

	// check if there are missing entries and test if this is allowed or not
	if !allowMissingVoters && len(actualVoters) != len(voters) {
		// create a list of all missing voters
		missing := make([]string, 0, len(voters))
		for voterName := range voters {
			if _, has := actualVoters[voterName]; !has {
				missing = append(missing, voterName)
			}
		}
		err = NewPollingSemanticError(nil, "the following voters are missing: %s", strings.Join(missing, ", "))
		return
	}

	if !allowMissingPolls && len(actualPolls) != len(polls) {
		// create a list of all missing polls
		missing := make([]string, 0, len(polls))
		for pollName := range polls {
			if _, has := actualPolls[pollName]; !has {
				missing = append(missing, pollName)
			}
		}
		err = NewPollingSemanticError(nil, "the following polls are missing: %s", strings.Join(missing, ", "))
		return
	}

	// make sure that each poll has a parser and a policy
	for pollName := range actualPolls {
		if _, hasParser := parsers[pollName]; !hasParser {
			err = NewPollingSemanticError(nil, "there is no parser for poll %s", pollName)
			return
		}

		if _, hasPolicy := policies[pollName]; !hasPolicy {
			err = NewPollingSemanticError(nil, "there is no policy for poll %s", pollName)
			return
		}
	}

	// now insert
	err = m.fillAllPolls(actualVoters, actualPolls, parsers, policies)
	return
}
