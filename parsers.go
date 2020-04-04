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
	"strings"
)

// PollingSyntaxError is an error returned if a syntax error is encountered.
//
// It can wrap another error (set to nil if not required) and has an optional line number, if this number is < 0
// the line number is assumed to be unknown / not existing for this error.
type PollingSyntaxError struct {
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
		errMessage = "syntax error: "
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

// isIgnoredLine tests if a line should be ignored during parsing, this happens if the line is empty or starts with #.
func isIgnoredLine(line string) bool {
	line = strings.TrimSpace(line)
	return line == "" || strings.HasPrefix(line, "#")
}

// votersLineRx is the regex used to parse a voter line, see ParseVotersLine.
var votersLineRx = regexp.MustCompile(`^\s*[*]\s+(.+?):\s*(\d+)\s*$`)

// ParseVotersLine parses a voter line.
//
// Line must be of the form "* <VOTER-NAME>: <WEIGHT>".
// The name can consist of arbitrary letters, weight must be a positive integer.
func ParseVotersLine(s string) (*Voter, error) {
	match := votersLineRx.FindStringSubmatch(s)
	if len(match) == 0 {
		return nil, NewPollingSyntaxError(nil, "voter line must be of the form \"* voter: weight\"")
	}
	name, weightString := match[1], match[2]
	weight, weightErr := ParseWeight(weightString)
	if weightErr != nil {
		return nil, NewPollingSyntaxError(weightErr, "voter line does not contain a valid integer (got %s)", weightString)
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
// Empty lines and lines starting with "#" are ignored.
func ParseVoters(r io.Reader) ([]*Voter, error) {
	scanner := bufio.NewScanner(r)
	lineNum := 0
	res := make([]*Voter, 0)
	for scanner.Scan() {
		lineNum++
		line := scanner.Text()
		// first test if the line should be ignored
		if !isIgnoredLine(line) {
			// should not be ignored, must be a valid voter
			voter, voterErr := ParseVotersLine(line)
			if voterErr != nil {
				return nil, convertParserErr(voterErr, lineNum)
			}
			res = append(res, voter)
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return res, nil
}

// ParseVotersFromString works like ParseVoters but reads from a string.
func ParseVotersFromString(s string) ([]*Voter, error) {
	reader := strings.NewReader(s)
	return ParseVoters(reader)
}

// parsing a description
var headLineRx = regexp.MustCompile(`^\s*#\s+(.+?)\s*$`)
var groupLineRx = regexp.MustCompile(`^\s*##\s+(.+?)\s*$`)
var pollLineRx = regexp.MustCompile(`^\s*###\s+(.+?)\s*$`)
var optionLineRx = regexp.MustCompile(`^\s*[*]\s+(.+?)\s*$`)
var medianOptionLineRx = regexp.MustCompile(`^\s*[-]\s+(.+?)\s*$`)

func matchFirst(s string, rxs ...*regexp.Regexp) (int, []string) {
	for i, rx := range rxs {
		match := rx.FindStringSubmatch(s)
		if len(match) > 0 {
			return i, match
		}
	}
	return -1, nil
}

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

type parserContext struct {
	*PollSkeletonCollection
	lastPollName   string
	currencyParser CurrencyParser
}

func newParserContext(currencyParser CurrencyParser) *parserContext {
	return &parserContext{
		PollSkeletonCollection: NewPollSkeletonCollection(""),
		lastPollName:           "",
		currencyParser:         currencyParser,
	}
}

type stateHandleFunc func(line string, context *parserContext) (parserState, error)

func runSecureStateHandleFunc(f stateHandleFunc, line string, context *parserContext) (next parserState, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("internal parsing error: %s", r)
		}
	}()
	next, err = f(line, context)
	return
}

func ParseCollectionSkeletons(r io.Reader, currencyParser CurrencyParser) (*PollSkeletonCollection, error) {
	if currencyParser == nil {
		currencyParser = SimpleEuroHandler{}
	}
	// create context to pass around
	context := newParserContext(currencyParser)
	// initial state is head
	state := headState
	// read lines from scanner
	scanner := bufio.NewScanner(r)
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := scanner.Text()
		// we can trim the line, no construct needs whitespaces in front / back
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// find out which handler to call
		var handler stateHandleFunc
		switch state {
		case headState:
			handler = handleHeadState
		case groupState:
			handler = handleGroupState
		case pollState:
			handler = handlePollState
		case optionState:
			handler = handleOptionState
		case groupOrPollState:
			handler = handleGroupOrPollState
		case optionalOptionState:
			handler = handleOptionalOptionState
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

func ParseCollectionSkeletonsFromString(currencyParser CurrencyParser, s string) (*PollSkeletonCollection, error) {
	r := strings.NewReader(s)
	return ParseCollectionSkeletons(r, currencyParser)
}

func handleHeadState(line string, context *parserContext) (parserState, error) {
	match := headLineRx.FindStringSubmatch(line)
	if len(match) == 0 {
		return invalidState, NewPollingSyntaxError(nil, "invalid head line, must be of form \"# <TITLE>\"")
	}
	if context.Title != "" {
		panic("Internal error: Expected that no title was set yet!")
	}
	context.Title = match[1]
	return groupState, nil
}

func handleGroupState(line string, context *parserContext) (parserState, error) {
	match := groupLineRx.FindStringSubmatch(line)
	if len(match) == 0 {
		return invalidState, NewPollingSyntaxError(nil, "invalid group line, must be of the form \"## <GROUP>\"")
	}
	group := NewPollGroup(match[1])
	context.Groups = append(context.Groups, group)
	return pollState, nil
}

func handlePollState(line string, context *parserContext) (parserState, error) {
	match := pollLineRx.FindStringSubmatch(line)
	if len(match) == 0 {
		return invalidState, NewPollingSyntaxError(nil, "invalid poll line, must be of the form \"### <POLL>\"")
	}
	context.lastPollName = match[1]
	return optionState, nil
}

func handleOptionState(line string, context *parserContext) (parserState, error) {
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
		group.Skeletons = append(group.Skeletons, skeleton)
		return optionalOptionState, nil
	case 1:
		// try to parse currency with parser from context
		currency, currencyErr := context.currencyParser.Parse(match[1])
		if currencyErr != nil {
			return invalidState, NewPollingSyntaxError(currencyErr, "Can't parse money value")
		}
		// add a new skeleton
		skeleton := NewMoneyPollSkeleton(context.lastPollName, currency)
		group.Skeletons = append(group.Skeletons, skeleton)
		return groupOrPollState, nil
	default:
		panic("Internal error: matchFirst returned an invalid index")
	}
}

func handleGroupOrPollState(line string, context *parserContext) (parserState, error) {
	// first try group, if this fails (err != nil) try poll state
	// note that these methods don't change the context if err != nil, so this is fine
	groupRes, groupErr := handleGroupState(line, context)
	if groupErr == nil {
		// success
		return groupRes, nil
	}
	// not a group, then try poll
	pollRes, pollErr := handlePollState(line, context)
	if pollErr == nil {
		return pollRes, nil
	}
	// both failed, raise an error
	return invalidState, NewPollingSyntaxError(nil, "expected either group or poll")
}

func handleOptionalOptionState(line string, context *parserContext) (parserState, error) {
	// now we have to parse either another option for the poll or a new group or a new poll
	// we use the other handler function for this (handleGroupOrPollState)
	// note that handleGroupOrPollState doesn't change the context if err != nil, so this is fine

	// first try to parse another option
	match := optionLineRx.FindStringSubmatch(line)
	if len(match) > 0 {
		// just append to last poll
		poll := context.getLastPollGroup().getLastPoll()
		poll.Options = append(poll.Options, match[1])
		return optionalOptionState, nil
	}
	// now it must be group or new poll
	handleRes, handleErr := handleGroupOrPollState(line, context)
	if handleErr == nil {
		// everything okay
		return handleRes, nil
	}
	// error
	return invalidState, NewPollingSyntaxError(nil, "expected either poll option, group or poll")
}
