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
	"fmt"
	"reflect"
)

// AbstractPoll describes any poll.
// It has a method PollType which returns the type as a string.
// Most operations dealing with polls do type assertions / switches are operate depending on the string of PollType().
//
// Constants are defined for implemented poll types: MedianPollType, SchulzePollType and BasicPollType.
//
// The method AddVote should add a would to the poll and return an error if the type is not supported
// (of type PollError).
// This method is not allowed to be called concurrently by multiple goroutines for the same poll
// instance.
//
// It is also recommended to implement the VoteGenerator interface to create votes for Aye, No and Abstention
// for a given poll.
type AbstractPoll interface {
	PollType() string
	AddVote(vote AbstractVote) error
}

// PollMap is a mapping from poll name to the poll with that name.
type PollMap map[string]AbstractPoll

const (
	MedianPollType  = "median-poll"
	SchulzePollType = "schulze-poll"
	BasicPollType   = "basic-poll"
)

// PollTypeError is an error returned if a skeleton / poll has an invalid  / unsupported type, for example if a
// skeleton can't be converted to an empty poll.
type PollTypeError struct {
	PollError
	Msg string
}

// NewPollTypeError returns a new PollTypeError given a format string and the values for the
// placeholders (like fmt.Sprintf).
func NewPollTypeError(msg string, a ...interface{}) PollTypeError {
	return PollTypeError{
		Msg: fmt.Sprintf(msg, a...),
	}
}

func (err PollTypeError) Error() string {
	return err.Msg
}

// VoteGenerator is used to describe polls that can produce a poll specific vote type for a basic answer
// (yes, no or abstention).
//
// It is not allowed to return a nil vote and error = nil, that is if there is no error the returned
// vote is not allowed to be nil.
//
// It should return a PollTypeError if an answer is not supported (or none at all).
// All polls implemented at the moment also implement this interface.
type VoteGenerator interface {
	AbstractPoll
	GenerateVoteFromBasicAnswer(voter *Voter, answer BasicPollAnswer) (AbstractVote, error)
}

// SkeletonConverter is a function that takes a skeleton and returns an empty poll for this skeleton.
// If an unknown type is encountered or the skeleton is in some way invalid it should return nil an error of type
// PollTypeError.
//
// An implementation is given in DefaultSkeletonConverter and a generator in NewDefaultSkeletonConverter.
type SkeletonConverter func(skel AbstractPollSkeleton) (AbstractPoll, error)

// NewDefaultSkeletonConverter is a generator function that returns a new SkeletonConverter.
// It does the following translations:
// A MoneyPollSkel gets translated to a MedianPol, it checks if the value described is >= (< 0 is not allowed).
// A PollSkeleton is translated to a BasicPoll or SchulzePoll.
// A BasicPoll is returned if the PollSkeleton has exactly two options,otherwise a SchulzePoll is created.
// If the number of options in the PollSkeleton is < 2 an error is returned.
//
// If convertToBasic is false a SchulzePoll will be returned even for two options.
//
// Note: A poll with two options is independent of the actual content of the two options, it is assumed that the first
// option represents Aye/Yes in some way and the second one No.
func NewDefaultSkeletonConverter(convertToBasic bool) SkeletonConverter {
	return func(skel AbstractPollSkeleton) (AbstractPoll, error) {
		return detaultSkeletonConverterGenerator(convertToBasic, skel)
	}
}

// DefaultSkeletonConverter is the default implementation of SkeletonConverter.
// It does the following translations:
// A MoneyPollSkel gets translated to a MedianPol, it checks if the value described is >= (< 0 is not allowed).
// A PollSkeleton is translated to a BasicPoll or SchulzePoll.
// A BasicPoll is returned if the PollSkeleton has exactly two options,otherwise a SchulzePoll is created.
// If the number of options in the PollSkeleton is < 2 an error is returned.
//
// It is just NewDefaultSkeletonConverter(true).
var DefaultSkeletonConverter = NewDefaultSkeletonConverter(true)

func detaultSkeletonConverterGenerator(convertToBasic bool, skel AbstractPollSkeleton) (AbstractPoll, error) {
	switch typedSkel := skel.(type) {
	case *MoneyPollSkeleton:
		value := typedSkel.Value
		if value.ValueCents < 0 {
			return nil,
				NewPollTypeError("value for median poll (\"%s\") is not allowed to be < 0! got %d for poll \"%s\"",
					value.ValueCents, typedSkel.Name)
		}
		return NewMedianPoll(MedianUnit(value.ValueCents), make([]*MedianVote, 0, defaultVotesSize)), nil

	case *PollSkeleton:
		numOptions := len(typedSkel.Options)
		switch numOptions {
		case 0, 1:
			return nil,
				NewPollTypeError("got only %d options, but at least two options are required. poll is \"%s\"",
					numOptions, typedSkel.Name)
		case 2:
			if convertToBasic {
				return NewBasicPoll(make([]*BasicVote, 0, defaultVotesSize)), nil
			}
			fallthrough
		default:
			return NewSchulzePoll(numOptions, make([]*SchulzeVote, 0, defaultVotesSize)), nil
		}
	default:
		return nil, NewPollTypeError("only money polls (median) and basic polls (e.g. normal poll, schulze are supported). Got type %s",
			reflect.TypeOf(skel))
	}
}

// ConvertSkeletonsToPolls does the translation from a list of skeletons to a list of (empty) polls.
// It uses a SkeletonConverter function to do the actual conversion and returns an error if any of the skeletons
// in the list is not "valid".
// If converterFunction is nil DefaultSkeletonConverter is used.
//
// ConvertSkeletonsToPolls is a function that does the same for maps.
func ConvertSkeletonsToPolls(skeletons []AbstractPollSkeleton, converterFunction SkeletonConverter) ([]AbstractPoll, error) {
	if converterFunction == nil {
		converterFunction = DefaultSkeletonConverter
	}
	res := make([]AbstractPoll, len(skeletons))

	for i, skeleton := range skeletons {
		emptyPoll, pollErr := converterFunction(skeleton)
		if pollErr != nil {
			return nil, pollErr
		}
		res[i] = emptyPoll
	}

	return res, nil
}

// ConvertSkeletonMapToEmptyPolls does the translation from a skeleton mapping to a map of (empty) polls.
// It uses a SkeletonConverter function to do the actual conversion and returns an error if any of the skeletons
// in the map is not "valid".
// If converterFunction is nil DefaultSkeletonConverter is used.
//
// ConvertSkeletonsToPolls is a function that does the same for lists.
func ConvertSkeletonMapToEmptyPolls(skeletons PollSkeletonMap, converterFunction SkeletonConverter) (PollMap, error) {
	if converterFunction == nil {
		converterFunction = DefaultSkeletonConverter
	}
	res := make(PollMap, len(skeletons))

	for name, skeleton := range skeletons {
		emptyPoll, pollErr := converterFunction(skeleton)
		if pollErr != nil {
			return nil, pollErr
		}
		res[name] = emptyPoll
	}

	return res, nil
}
