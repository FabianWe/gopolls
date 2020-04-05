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

type AbstractPoll interface {
	PollType() string
}

const (
	MedianPollType  = "median-poll"
	SchulzePollType = "schulze-poll"
	BasicPollType   = "basic-poll"
)

type SkelTypeConversionError string

func NewSkelTypeConversionError(msg string, a ...interface{}) SkelTypeConversionError {
	return SkelTypeConversionError(fmt.Sprintf(msg, a...))
}

func (err SkelTypeConversionError) Error() string {
	return string(err)
}

type SkeletonConverter func(skel AbstractPollSkeleton) (AbstractPoll, error)

func NewDefaultSkeletonConverter(convertToBasic bool) SkeletonConverter {
	return func(skel AbstractPollSkeleton) (AbstractPoll, error) {
		return detaultSkeletonConverterGenerator(convertToBasic, skel)
	}
}

var DefaultSkeletonConverter = NewDefaultSkeletonConverter(true)

func detaultSkeletonConverterGenerator(convertToBasic bool, skel AbstractPollSkeleton) (AbstractPoll, error) {
	defaultVotesSize := 50
	switch typedSkel := skel.(type) {
	case *MoneyPollSkeleton:
		value := typedSkel.Value
		if value.ValueCents < 0 {
			return nil,
				NewSkelTypeConversionError("value for median poll is not allowed to be < 0! got %d for poll %s",
					value.ValueCents, typedSkel.Name)
		}
		return NewMedianPoll(MedianUnit(value.ValueCents), make([]*MedianVote, 0, defaultVotesSize)), nil

	case *PollSkeleton:
		numOptions := len(typedSkel.Options)
		switch numOptions {
		case 0, 1:
			return nil,
				NewSkelTypeConversionError("Got only %d options, but at least two options are required", numOptions)
		case 2:
			if convertToBasic {
				return NewBasicPoll(make([]*BasicVote, 0, defaultVotesSize)), nil
			}
			fallthrough
		default:
			return NewSchulzePoll(numOptions, make([]*SchulzeVote, 0, defaultVotesSize)), nil
		}
	default:
		return nil, NewSkelTypeConversionError("Only money polls (median) and basic polls (e.g. normal poll, scholze are supported). Got type %s",
			reflect.TypeOf(skel))
	}
}

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
