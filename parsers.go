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
	"bufio"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"
)

///// ERRORS /////

// PollingSyntaxError is an error returned if a syntax error is encountered.
//
// It can wrap another error (set to nil if not required) and has an optional line number, if this number is < 0
// the line number is assumed to be unknown / not existing for this error.
type PollingSyntaxError struct {
	PollError
	Err     error
	Msg     string
	LineNum int
}

// NewPollingSyntaxError returns a new PollingSyntaxError with a line number of -1.
//
// The message can be formatted with placeholders (like fmt.Sprintf).
func NewPollingSyntaxError(err error, msg string, a ...interface{}) PollingSyntaxError {
	return PollingSyntaxError{
		Err:     err,
		Msg:     fmt.Sprintf(msg, a...),
		LineNum: -1,
	}
}

// WithLineNum returns a copy of the error but with the line number set to a new value.
func (err PollingSyntaxError) WithLineNum(lineNum int) PollingSyntaxError {
	return PollingSyntaxError{
		Err:     err.Err,
		Msg:     err.Msg,
		LineNum: lineNum,
	}
}

// convertParserErr wraps a call to PollingSyntaxError.WithLineNum if err is of type PollingSyntaxError.
// We don't use errors.Is here because we want the exact type.
func convertParserErr(err error, lineNum int) error {
	if err == nil {
		return nil
	}
	switch e := err.(type) {
	case PollingSyntaxError:
		return e.WithLineNum(lineNum)
	default:
		return err
	}
}

// Error returns the error message, it contains (if given) the line number and error cause (the wrapped error) and the
// original message.
func (err PollingSyntaxError) Error() string {
	errMessage := ""
	if err.LineNum < 0 {
		errMessage = ""
	} else {
		errMessage = fmt.Sprintf("syntax error in line %d: ", err.LineNum)
	}
	errMessage += err.Msg
	if err.Err != nil {
		errMessage = errMessage + " Caused by: " + err.Err.Error()
	}
	return errMessage
}

// Unwrap returns the wrapped error.
func (err PollingSyntaxError) Unwrap() error {
	return err.Err
}

// PollingSemanticError is an error returned if somewhere an option that is syntactically correct is
// parsed but is not valid semantically.
//
// it can wrap another error (set to nil of not required).
type PollingSemanticError struct {
	PollError
	Err error
	Msg string
}

// NewPollingSemanticError returns a new PollingSemanticError.
//
// The message can be formatted with placeholders (like fmt.Sprintf).
func NewPollingSemanticError(err error, msg string, a ...interface{}) PollingSemanticError {
	return PollingSemanticError{
		Err: err,
		Msg: fmt.Sprintf(msg, a...),
	}
}

func (err PollingSemanticError) Error() string {
	errMessage := err.Msg

	if err.Err != nil {
		errMessage = errMessage + " Caused by: " + err.Err.Error()
	}
	return errMessage
}

// Unwrap returns the wrapped error.
func (err PollingSemanticError) Unwrap() error {
	return err.Err
}

// ParserValidationError is an error returned if a validation of the input files.
// Such errors include: invalid utf-8 encoding (see ErrInvalidEncoding) or a line was longer than allowed.
type ParserValidationError struct {
	PollError
	Message string
}

func NewParserValidationError(msg string) *ParserValidationError {
	return &ParserValidationError{
		Message: msg,
	}
}

func (err ParserValidationError) Error() string {
	return "validation of parser input failed: " + err.Message
}

func (err ParserValidationError) Unwrap() error {
	return nil
}

// ErrInvalidEncoding is an error used to signal that an input string is not encoded with valid utf-8.
var ErrInvalidEncoding = NewParserValidationError("invalid utf-8 encoding in input")

///// PARSERS /////

// isIgnoredLine tests if a line should be ignored during parsing, this happens if the line is empty or starts with #.
func isIgnoredLine(line string) bool {
	line = strings.TrimSpace(line)
	return line == "" || strings.HasPrefix(line, "#")
}

// votersLineRx is the regex used to parse a voter line, see ParseVotersLine.
var votersLineRx = regexp.MustCompile(`^\s*[*]\s+(.+?)\s*(?::\s+(\d+)\s*)?$`)

// VotersParser parses voters from a file / string.
// See ParseVotersLine and ParseVoters for details.
//
// Furthermore the parser can be configured to read only a certain amount of voters or validate / limit the file.
// This limit / validation is set via the member variables above. They all default to a value that disables all limits
// and checks in NewVotersParser. The default value is -1 for all int types and NoWeight for MaxVotersWeight.
//
// This checking / limitation make it easier to already prevent entries with too many values from parsing. It also
// gives an easy method to disallow files that are too big from being parsed.
// All these validations are indicated by a returned error of type ParserValidationError.
//
// The meaning is as follows: MaxNumLines is the number of lines that are allowed in a voters file for ParseVoters.
// MaxNumVoters is the number of voters that are allowed to be parsed in ParseVoters.
// Note that we allow comments and empty lines in the file, thus we have one variable for lines and one for voters.
//
// MaxLineLength is the maximal number of bytes (not runes) allowed in a single line of the file.
// MaxVotersNameLength is the maximal number of bytes allowed in a single voters name.
// MaxVotersWeight is the maximal weight a voter can have, this is useful to for example avoid overflows when you have
// many voters.
//
// However MaxLineLength is probably one of the most useful limits because it finds very long lines early and
// avoids the parsing of such lines.
//
// Of course some combinations don't make sense, for example setting MaxLineLength=21 and MaxVotersNameLength=42
// will never result in a voter name length > 21.
//
// ComputeDefaultMaxLineLength is a small helper that may be called and sets MaxLineLength depending on
// MaxVotersNameLength and MaxVotersWeight.
type VotersParser struct {
	MaxNumLines         int
	MaxNumVoters        int
	MaxLineLength       int
	MaxVotersNameLength int
	MaxVotersWeight     Weight
}

// NewVotersParser returns a new parser with all limitations disabled.
func NewVotersParser() *VotersParser {
	return &VotersParser{
		MaxNumLines:         -1,
		MaxNumVoters:        -1,
		MaxLineLength:       -1,
		MaxVotersNameLength: -1,
		MaxVotersWeight:     NoWeight,
	}
}

// ComputeDefaultMaxLineLength sets MaxLineLength depending on the values of MaxVotersNameLength (if set) and
// MaxVotersWeight.
// It allows the whitespaces that are required in the description and adds a small constant to allow additional whitespaces,
// but not too many.
func (parser *VotersParser) ComputeDefaultMaxLineLength() {
	if parser.MaxNumLines < 0 {
		return
	}
	parser.MaxLineLength = parser.MaxVotersNameLength + len(strconv.FormatUint(uint64(parser.MaxVotersWeight), 10)) + 4 + 16
}

// ParseVotersLine parses a voter line.
//
// Line must be of the form "* <VOTER-NAME>: <WEIGHT>".
// The name can consist of arbitrary letters, weight must be a positive integer.
// The weight can also be omitted and defaults to 1.
// The returned error will be of type ParserValidationError or PollingSyntaxError.
func (parser *VotersParser) ParseVotersLine(s string) (*Voter, error) {
	// first validate that s is valid utf-8
	if !utf8.ValidString(s) {
		return nil, ErrInvalidEncoding
	}
	// validate length if max line length is set
	if parser.MaxLineLength >= 0 {
		// check number of bytes here, not number of runes!
		if len(s) > parser.MaxLineLength {
			return nil, NewParserValidationError(fmt.Sprintf("line is too long: got line of length %d, allowed max length is %d",
				len(s), parser.MaxLineLength))
		}
	}
	match := votersLineRx.FindStringSubmatch(s)
	if len(match) == 0 {
		return nil, NewPollingSyntaxError(nil, "voter line must be of the form \"* voter: weight\"")
	}
	name, weightString := match[1], match[2]
	name = strings.TrimSpace(name)
	weightString = strings.TrimSpace(weightString)
	var weight Weight
	var weightErr error
	if weightString == "" {
		weight = 1
	} else {
		weight, weightErr = ParseWeight(weightString)
	}

	if weightErr != nil {
		return nil, NewPollingSyntaxError(weightErr, "voter line does not contain a valid integer (got %s)", weightString)
	}

	// now validate lengths
	if parser.MaxVotersNameLength >= 0 {
		nameLength := utf8.RuneCountInString(name)
		if nameLength > parser.MaxVotersNameLength {
			return nil, NewParserValidationError(fmt.Sprintf("voter name is too long: got length %d, allowed max length is %d",
				nameLength, parser.MaxVotersNameLength))
		}
	}

	if parser.MaxVotersWeight != NoWeight && weight > parser.MaxVotersWeight {
		return nil, NewParserValidationError(fmt.Sprintf("voter weight is too big, got %d but max allowed length is %d",
			weight, parser.MaxVotersWeight))
	}
	res := Voter{
		Name:   name,
		Weight: weight,
	}
	return &res, nil
}

// ParseVoters parses a list of voters from a reader.
//
// Each line must contain one voter entry. Each line must be of the form as described in ParseVotersLine, in short
//
// "* <VOTER-NAME>: <WEIGHT>".
//
// or
//
// "* <VOTER-NAME>"
//
// in which case weight defaults to 1.
//
// Empty lines and lines starting with "#" are ignored.
//
// This method will return an internal error whenever for syntax errors / validation errors, all errors from reader are
// returned directly however.
//
// The returned internals errors are either PollingSyntaxError or ParserValidationError.
func (parser *VotersParser) ParseVoters(r io.Reader) ([]*Voter, error) {
	scanner := bufio.NewScanner(r)
	// if a max line length is set create a buffer with that max length
	if parser.MaxLineLength >= 0 {
		// set max length of the buffer to that number
		// the initial size of the buffer will be 4096, but if max length < 4096 we set it to that
		buffLength := 4096
		if parser.MaxLineLength < 4096 {
			buffLength = parser.MaxLineLength
		}
		buff := make([]byte, buffLength)
		scanner.Buffer(buff, parser.MaxLineLength)
	}
	lineNum := 0
	res := make([]*Voter, 0)
	for scanner.Scan() {
		lineNum++
		if parser.MaxNumLines >= 0 && lineNum > parser.MaxNumLines {
			return nil, NewParserValidationError(fmt.Sprintf("there are too many lines: only %d lines in voters files are allowed", parser.MaxNumLines))
		}
		line := scanner.Text()
		// first test if the line should be ignored
		if !isIgnoredLine(line) {
			// should not be ignored, must be a valid voter
			voter, voterErr := parser.ParseVotersLine(line)
			if voterErr != nil {
				return nil, convertParserErr(voterErr, lineNum)
			}
			res = append(res, voter)
			if parser.MaxNumVoters >= 0 && len(res) > parser.MaxNumVoters {
				return nil, NewParserValidationError(fmt.Sprintf("there are too many voters: only %d voters are allowed", parser.MaxNumVoters))
			}
		}
	}
	if err := scanner.Err(); err != nil {
		// if the error is that the line is too long return it as an validation error
		if errors.Is(err, bufio.ErrTooLong) {
			var errString string
			if parser.MaxLineLength >= 0 {
				errString = fmt.Sprintf("line is too long: max allowed number of bytes in line is %d",
					parser.MaxLineLength)
			} else {
				errString = "line is too long: max number of bytes is determined by go scanner buffer size (probably 4096)"
			}
			return nil, NewParserValidationError(errString)
		}
		return nil, err
	}
	return res, nil
}

// ParseVotersFromString works like ParseVoters but reads from a string.
func (parser *VotersParser) ParseVotersFromString(s string) ([]*Voter, error) {
	reader := strings.NewReader(s)
	return parser.ParseVoters(reader)
}

// parsing a description

// the following regular expressions are used while parsing the input file
var headLineRx = regexp.MustCompile(`^\s*#\s+(.+?)\s*$`)
var groupLineRx = regexp.MustCompile(`^\s*##\s+(.+?)\s*$`)
var pollLineRx = regexp.MustCompile(`^\s*###\s+(.+?)\s*$`)
var optionLineRx = regexp.MustCompile(`^\s*[*]\s+(.+?)\s*$`)
var medianOptionLineRx = regexp.MustCompile(`^\s*[-]\s+(.+?)\s*$`)

// matchFirst tries to match s against each regex.
// It returns the index of the first match and the complete match (from rx.FindStringSubmatch).
// If no regex matches it returns -1 and nil.
func matchFirst(s string, rxs ...*regexp.Regexp) (int, []string) {
	for i, rx := range rxs {
		match := rx.FindStringSubmatch(s)
		if len(match) > 0 {
			return i, match
		}
	}
	return -1, nil
}

// parserState is the state in which the parser is, it starts in headState and switches states depending on what
// was parsed in the last run.
type parserState int8

const (
	// expect the title (#)
	headState parserState = iota
	// expect a group (##)
	groupState
	// expect a poll name (###)
	pollState
	// expect an option (schulze or median, so * or -)
	// we're in this state right after reading a poll name
	optionState
	// expect either a new group or a poll
	groupOrPollState
	// expect another option, if any
	optionalOptionState
	// invalid state from which we should never continue
	invalidState
)

// parserContext stores information passed around while parsing an input.
type parserContext struct {
	*PollSkeletonCollection
	lastPollName   string
	currencyParser CurrencyParser
	numSkels       int
}

func newParserContext(currencyParser CurrencyParser) *parserContext {
	return &parserContext{
		PollSkeletonCollection: NewPollSkeletonCollection(""),
		lastPollName:           "",
		currencyParser:         currencyParser,
		numSkels:               0,
	}
}

// stateHandleFunc is a function that is applied to a certain line and tests if the line meets the expectations.
// If the line is of the wrong format it returns an error != nil.
type stateHandleFunc func(line string, context *parserContext) (parserState, error)

// runSecureStateHandleFunc wraps a call to f and recovers from any panic that might occur.
// If a handler panics (which it shouldn't) this panic is fetched and returned as an error.
func runSecureStateHandleFunc(f stateHandleFunc, line string, context *parserContext) (next parserState, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("internal parsing error: %s", r)
		}
	}()
	next, err = f(line, context)
	return
}

// PollCollectionParser parses a poll collection from a file / string.
// See ParseCollectionSkeletons and ParseCollectionSkeletonsFromString.
//
// Furthermore the parser can be configured to read only a certain amount of lines / put restrictions on the polls parsed.
// This is the same idea as in VotersParser, see there for details of when you would want to use restrictions.
//
// A new parser from NewPollCollectionParser sets all values to -1, meaning no restrictions.
//
// The following restrictions can be configured:
// MaxNumLines is the number of lines that are allowed in a polls file.
// MaxNumPolls is the maximal number of polls allowed in that file.
// MaxLineLength is the maximal number of bytes (not runes) allowed in a single line of the file.
// MaxTitleLength is the maximal length the title / heading is allowed to have.
// MaxGroupNameLength is the maximal length a group is allowed to have.
// MaxPollNameLength is the maximal length a poll name is allowed to have.
// MaxNumOptions should be set to at least two, it describes how many options in a basic poll are allowed.
// MaxOptionLength is the maximal length a single option is allowed to have.
// MaxCurrencyValue is the maximal currency value (in cents) that is allowed. This can be useful to avoid overflows /
// database limitations.
//
// Again, some combinations would not make sense, like setting MaxNumLines=21 and MaxTitleLength=42.
type PollCollectionParser struct {
	MaxNumLines        int
	MaxNumPolls        int
	MaxLineLength      int
	MaxTitleLength     int
	MaxGroupNameLength int
	MaxPollNameLength  int
	MaxNumOptions      int
	MaxOptionLength    int
	MaxCurrencyValue   int
}

// NewPollCollectionParser returns a new parser with all limitations / restrictions disabled.
func NewPollCollectionParser() *PollCollectionParser {
	return &PollCollectionParser{
		MaxNumLines:        -1,
		MaxNumPolls:        -1,
		MaxLineLength:      -1,
		MaxTitleLength:     -1,
		MaxGroupNameLength: -1,
		MaxPollNameLength:  -1,
		MaxNumOptions:      -1,
		MaxOptionLength:    -1,
		MaxCurrencyValue:   -1,
	}
}

func (parser *PollCollectionParser) validateLine(line string, lineNum int) error {
	if parser.MaxNumLines >= 0 && lineNum > parser.MaxNumLines {
		return NewParserValidationError(fmt.Sprintf("there are too many lines: only %d lines in polls file are allowed", parser.MaxNumLines))
	}
	if !utf8.ValidString(line) {
		return ErrInvalidEncoding
	}
	if parser.MaxLineLength >= 0 {
		// check number of bytes here, not number of runes!
		if len(line) > parser.MaxLineLength {
			return NewParserValidationError(fmt.Sprintf("line is too long: got line of length %d, allowed max length is %d",
				len(line), parser.MaxLineLength))
		}
	}
	return nil
}

func (parser *PollCollectionParser) setupScanner(r io.Reader) *bufio.Scanner {
	scanner := bufio.NewScanner(r)
	// max line length is set create a buffer with that max length
	if parser.MaxLineLength >= 0 {
		// set max length of the buffer to that number
		// the initial size of the buffer will be 4096, but if max length < 4096 we set it to that
		buffLength := 4096
		if parser.MaxLineLength < 4096 {
			buffLength = parser.MaxLineLength
		}
		buff := make([]byte, buffLength)
		scanner.Buffer(buff, parser.MaxLineLength)
	}
	return scanner
}

// ParseCollectionSkeletons parses a collection of poll descriptions and returns them as skeletons.
// See wiki and example files for format details.
func (parser *PollCollectionParser) ParseCollectionSkeletons(r io.Reader, currencyParser CurrencyParser) (*PollSkeletonCollection, error) {
	if currencyParser == nil {
		currencyParser = SimpleEuroHandler{}
	}
	// create context to pass around
	context := newParserContext(currencyParser)
	// initial state is head
	state := headState
	// read lines from scanner
	scanner := parser.setupScanner(r)
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := scanner.Text()
		if validateLineErr := parser.validateLine(line, lineNum); validateLineErr != nil {
			return nil, validateLineErr
		}
		// we can trim the line, no construct needs whitespaces in front / back
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// find out which handler to call
		var handler stateHandleFunc
		switch state {
		case headState:
			handler = parser.handleHeadState
		case groupState:
			handler = parser.handleGroupState
		case pollState:
			handler = parser.handlePollState
		case optionState:
			handler = parser.handleOptionState
		case groupOrPollState:
			handler = parser.handleGroupOrPollState
		case optionalOptionState:
			handler = parser.handleOptionalOptionState
		default:
			return nil, errors.New("internal error: Parser entered an invalid state")
		}
		// call handler and also recover from all panics
		nextState, stateErr := runSecureStateHandleFunc(handler, line, context)
		if stateErr != nil {
			return nil, convertParserErr(stateErr, lineNum)
		}
		state = nextState
	}
	if scanErr := scanner.Err(); scanErr != nil {
		// if the error is that th line is too long return it as an validation error
		if errors.Is(scanErr, bufio.ErrTooLong) {
			var errString string
			if parser.MaxLineLength >= 0 {
				errString = fmt.Sprintf("line is too long: max allowed number of bytes in line is %d",
					parser.MaxLineLength)
			} else {
				errString = "line is too long: max number of bytes is determined by go scanner buffer size (probably 4096)"
			}
			return nil, NewParserValidationError(errString)
		}
		return nil, scanErr
	}

	// no test if in all "basic" skeletons there are at least two options, everything
	// else doesn't make sense
	res := context.PollSkeletonCollection

	for _, group := range res.Groups {
		for _, pollSkel := range group.Skeletons {
			if asPollSkel, ok := pollSkel.(*PollSkeleton); ok {
				if len(asPollSkel.Options) < 2 {
					// Not really syntax related (kind of if the formal syntax would specifically say
					// two), but anyway, should be fine
					return nil, NewPollingSyntaxError(nil, "poll \"%s\" contains only %d options, expected at most 2",
						asPollSkel.Name, len(asPollSkel.Options))
				}
			}
		}
	}

	// now test if we're in a not valid end state
	switch state {
	case headState:
		return nil, NewPollingSyntaxError(nil, "no title found \"# <TITLE>\"")
	case optionState:
		return nil, NewPollingSyntaxError(nil, "found beginning of a poll but no option was given")
	}

	return res, nil
}

// ParseCollectionSkeletonsFromString works as ParseCollectionSkeletons but parses the input from a string.
func (parser *PollCollectionParser) ParseCollectionSkeletonsFromString(currencyParser CurrencyParser, s string) (*PollSkeletonCollection, error) {
	r := strings.NewReader(s)
	return parser.ParseCollectionSkeletons(r, currencyParser)
}

func (parser *PollCollectionParser) validateTitle(title string) error {
	if parser.MaxTitleLength >= 0 && len(title) > parser.MaxTitleLength {
		return NewParserValidationError(fmt.Sprintf("title is too long: got length %d, allowed max length is %d",
			len(title), parser.MaxTitleLength))
	}
	return nil
}

func (parser *PollCollectionParser) handleHeadState(line string, context *parserContext) (parserState, error) {
	match := headLineRx.FindStringSubmatch(line)
	if len(match) == 0 {
		return invalidState, NewPollingSyntaxError(nil, "invalid head line, must be of form \"# <TITLE>\"")
	}
	if context.Title != "" {
		panic("Internal error: Expected that no title was set yet!")
	}
	context.Title = match[1]
	if titleValidationErr := parser.validateTitle(context.Title); titleValidationErr != nil {
		return invalidState, titleValidationErr
	}
	return groupState, nil
}

func (parser *PollCollectionParser) validateGroupName(name string) error {
	if parser.MaxGroupNameLength >= 0 && len(name) > parser.MaxGroupNameLength {
		return NewParserValidationError(fmt.Sprintf("group name is too long: got length %d, allowed max length is %d",
			len(name), parser.MaxGroupNameLength))
	}
	return nil
}

func (parser *PollCollectionParser) handleGroupState(line string, context *parserContext) (parserState, error) {
	match := groupLineRx.FindStringSubmatch(line)
	if len(match) == 0 {
		return invalidState, NewPollingSyntaxError(nil, "invalid group line, must be of the form \"## <GROUP>\"")
	}
	groupName := match[1]
	if groupNameValidationErr := parser.validateGroupName(groupName); groupNameValidationErr != nil {
		return invalidState, groupNameValidationErr
	}
	group := NewPollGroup(groupName)
	context.Groups = append(context.Groups, group)
	return pollState, nil
}

func (parser *PollCollectionParser) validatePollName(name string) error {
	if parser.MaxPollNameLength >= 0 && len(name) > parser.MaxPollNameLength {
		return NewParserValidationError(fmt.Sprintf("poll name is too long: got length %d, allowed max length is %d",
			len(name), parser.MaxPollNameLength))
	}
	return nil
}

func (parser *PollCollectionParser) handlePollState(line string, context *parserContext) (parserState, error) {
	match := pollLineRx.FindStringSubmatch(line)
	if len(match) == 0 {
		return invalidState, NewPollingSyntaxError(nil, "invalid poll line, must be of the form \"### <POLL>\"")
	}
	context.lastPollName = match[1]
	if nameValidationErr := parser.validatePollName(context.lastPollName); nameValidationErr != nil {
		return invalidState, nameValidationErr
	}
	return optionState, nil
}

func (parser *PollCollectionParser) validateNumPolls(numPolls int) error {
	if parser.MaxNumPolls >= 0 && numPolls > parser.MaxNumPolls {
		return NewParserValidationError(fmt.Sprintf("there are too many polls: only %d polls are allowed", parser.MaxNumPolls))
	}
	return nil
}

func (parser *PollCollectionParser) validateNewOption(options []string) error {
	last := options[len(options)-1]
	if parser.MaxOptionLength >= 0 && len(last) > parser.MaxOptionLength {
		return NewParserValidationError(fmt.Sprintf("poll option is too long: got length %d, allowed max length is %d",
			len(last), parser.MaxOptionLength))
	}
	if parser.MaxNumOptions >= 0 && len(options) > parser.MaxNumOptions {
		return NewParserValidationError(fmt.Sprintf("there are too many options in a poll: only %d options are allowed",
			parser.MaxNumOptions))
	}

	return nil
}

func (parser *PollCollectionParser) validateMoneyValue(value CurrencyValue) error {
	if parser.MaxCurrencyValue >= 0 && value.ValueCents > parser.MaxCurrencyValue {
		return NewParserValidationError(fmt.Sprintf("value for money poll is too big, got %d cents, max allowed cents is %d",
			value.ValueCents, parser.MaxCurrencyValue))
	}
	return nil
}

func (parser *PollCollectionParser) handleOptionState(line string, context *parserContext) (parserState, error) {
	// just some assertions to be sure
	if context.lastPollName == "" {
		panic("Internal error: Trying to parse poll option, but no poll was parsed first")
	}
	group := context.getLastPollGroup()
	// can be either schulze or median, try both
	index, match := matchFirst(line, optionLineRx, medianOptionLineRx)
	switch index {
	case -1:
		return invalidState, NewPollingSyntaxError(nil, "invalid option line, must either be a standard option \"*\" or money value \"-}")
	case 0:
		// add a new skeleton with this option
		skeleton := NewPollSkeleton(context.lastPollName)
		skeleton.Options = append(skeleton.Options, match[1])
		if validateOptionErr := parser.validateNewOption(skeleton.Options); validateOptionErr != nil {
			return invalidState, validateOptionErr
		}
		group.Skeletons = append(group.Skeletons, skeleton)
		context.numSkels++
		if numPollErr := parser.validateNumPolls(context.numSkels); numPollErr != nil {
			return invalidState, numPollErr
		}
		return optionalOptionState, nil
	case 1:
		// try to parse currency with parser from context
		currency, currencyErr := context.currencyParser.Parse(match[1])
		if currencyErr != nil {
			return invalidState, NewPollingSyntaxError(currencyErr, "Can't parse money value")
		}
		// only positive values are allowed
		// strictly speaking not a syntax error but fine
		if currency.ValueCents < 0 {
			return invalidState, NewPollingSemanticError(nil, "string %s describes a negative value, can't be used in a median poll", line)
		}
		// validate max value
		if currencyMaxValidationErr := parser.validateMoneyValue(currency); currencyMaxValidationErr != nil {
			return invalidState, currencyMaxValidationErr
		}
		// add a new skeleton
		skeleton := NewMoneyPollSkeleton(context.lastPollName, currency)
		group.Skeletons = append(group.Skeletons, skeleton)
		context.numSkels++
		if numPollErr := parser.validateNumPolls(context.numSkels); numPollErr != nil {
			return invalidState, numPollErr
		}
		return groupOrPollState, nil
	default:
		panic("Internal error: matchFirst returned an invalid index")
	}
}

func (parser *PollCollectionParser) handleGroupOrPollState(line string, context *parserContext) (parserState, error) {
	// first try group, if this fails (err != nil) try poll state
	// note that these methods don't change the context if err != nil, so this is fine
	groupRes, groupErr := parser.handleGroupState(line, context)
	if groupErr == nil {
		// success
		return groupRes, nil
	}
	// if it is a validation error: return the validation error and don't try
	// poll
	isValidationErrDummy := NewParserValidationError("")
	if errors.As(groupErr, &isValidationErrDummy) {
		return invalidState, groupErr
	}
	// not a group, then try poll
	pollRes, pollErr := parser.handlePollState(line, context)
	if pollErr == nil {
		return pollRes, nil
	}
	// again test for validation error
	if errors.As(pollErr, &isValidationErrDummy) {
		return invalidState, pollErr
	}
	// both failed, raise an error
	return invalidState, NewPollingSyntaxError(nil, "expected either group or poll")
}

func (parser *PollCollectionParser) handleOptionalOptionState(line string, context *parserContext) (parserState, error) {
	// now we have to parse either another option for the poll or a new group or a new poll
	// we use the other handler function for this (handleGroupOrPollState)
	// note that handleGroupOrPollState doesn't change the context if err != nil, so this is fine

	// first try to parse another option
	match := optionLineRx.FindStringSubmatch(line)
	if len(match) > 0 {
		// just append to last poll
		poll := context.getLastPollGroup().getLastPoll()
		poll.Options = append(poll.Options, match[1])
		if validateOptionErr := parser.validateNewOption(poll.Options); validateOptionErr != nil {
			return invalidState, validateOptionErr
		}
		return optionalOptionState, nil
	}
	// now it must be group or new poll
	handleRes, handleErr := parser.handleGroupOrPollState(line, context)
	if handleErr == nil {
		// everything okay
		return handleRes, nil
	}
	// error

	// again return validation errors unchanged
	isValidationErrDummy := NewParserValidationError("")
	if errors.As(handleErr, &isValidationErrDummy) {
		return invalidState, handleErr
	}
	return invalidState, NewPollingSyntaxError(nil, "expected either poll option, group or poll")
}
