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

// ParseVotersString works like ParseVoters but reads from a string.
func ParseVotersString(s string) ([]*Voter, error) {
	reader := strings.NewReader(s)
	return ParseVoters(reader)
}
