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
	"math"
	"strconv"
	"strings"
)

// Weight is the type used to reference voter weights.
type Weight uint32

// NoWeight is a value used to signal that a value is not a valid Weight, for example as default argument.
const NoWeight Weight = math.MaxUint32

// defaultVotesSize is the default capacity for objects that store a list of voters / elements for each voter.
const defaultVotesSize = 50

// ParseWeight parses a Weight from a string.
//
// A PollingSyntaxError is returned if s is no valid int or is NoWeight.
func ParseWeight(s string) (Weight, error) {
	asInt, err := strconv.ParseUint(s, 10, 32)
	if err != nil {
		return NoWeight, NewPollingSyntaxError(err, "")
	}
	res := Weight(asInt)
	if res == NoWeight {
		return NoWeight, NewPollingSyntaxError(nil, "integer value %d is too big", NoWeight)
	}
	return res, nil
}

// WeightMin returns the minimum of a and b.
func WeightMin(a, b Weight) Weight {
	if a < b {
		return a
	}
	return b
}

// WeightMax returns the maximum of a and b.
func WeightMax(a, b Weight) Weight {
	if a > b {
		return a
	}
	return b
}

// DuplicateError is an error returned if somewhere a duplicate name is found.
//
// For example two voter objects with the same name.
type DuplicateError struct {
	PollError
	Msg string
}

// NewDuplicateError returns a new DuplicateError.
func NewDuplicateError(msg string) DuplicateError {
	return DuplicateError{
		Msg: msg,
	}
}

func (err DuplicateError) Error() string {
	return err.Msg
}

// LowerStringSet is a set of lower case strings.
type LowerStringSet map[string]struct{}

// NewLowerStringSet returns a new set given its elements, all elements are transformed to lower case.
func NewLowerStringSet(elements []string) LowerStringSet {
	res := make(LowerStringSet, len(elements))
	for _, element := range elements {
		res[strings.ToLower(element)] = struct{}{}
	}
	return res
}

// Insert inserts a new element, the element is transformed to lower case.
func (s LowerStringSet) Insert(element string) {
	s[strings.ToLower(element)] = struct{}{}
}

// Extend adds all elements to the set, all elements are transformed to lower case.
func (s LowerStringSet) Extend(elements []string) {
	for _, element := range elements {
		s[strings.ToLower(element)] = struct{}{}
	}
}

// ContainsLowercase returns true if element is contained within s.
// Note that element must already be lower case, otherwise this method will not work correctly!
func (s LowerStringSet) ContainsLowercase(element string) bool {
	_, contains := s[element]
	return contains
}

// Contains returns true if the lowercase version of s is contained within s.
// The difference to ContainsLowercase is that this method will always convert s to lower case.
func (s LowerStringSet) Contains(element string) bool {
	_, contains := s[strings.ToLower(element)]
	return contains
}

func (s LowerStringSet) String() string {
	if len(s) == 0 {
		return "{}"
	}
	first := true
	var b strings.Builder
	b.WriteRune('{')
	for element := range s {
		if first {
			first = false
		} else {
			b.WriteString(", ")
		}
		b.WriteString(element)
	}
	b.WriteRune('}')
	return b.String()
}
