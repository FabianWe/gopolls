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

package tests

import (
	"github.com/FabianWe/gopolls"
	"testing"
)

func TestBasicPollOne(t *testing.T) {
	voterOne := gopolls.NewVoter("one", 1)
	voterTwo := gopolls.NewVoter("two", 2)
	voterThree := gopolls.NewVoter("three", 3)

	voteOne := gopolls.NewBasicVote(voterOne, gopolls.Aye)
	voteTwo := gopolls.NewBasicVote(voterTwo, gopolls.No)
	voteThree := gopolls.NewBasicVote(voterThree, gopolls.Abstention)

	poll := gopolls.NewBasicPoll([]*gopolls.BasicVote{voteOne, voteTwo, voteThree})

	res := poll.Tally()
	expectedVotes := gopolls.BasicPollCounter{
		NumNoes:       1,
		NumAyes:       1,
		NumAbstention: 1,
		NumInvalid:    0,
	}
	if !res.NumberVoters.Equals(&expectedVotes) {
		t.Errorf("Expected basic poll result to be %v, got %v instead",
			expectedVotes, *res.NumberVoters)
	}

	expectedWeightedVotes := gopolls.BasicPollCounter{
		NumNoes:       2,
		NumAyes:       1,
		NumAbstention: 3,
		NumInvalid:    0,
	}
	if !res.WeightedVotes.Equals(&expectedWeightedVotes) {
		t.Errorf("Expected basic poll result to be %v, got %v instead",
			expectedWeightedVotes, *res.WeightedVotes)
	}
}

func TestBasicPollTwo(t *testing.T) {
	voterOne := gopolls.NewVoter("one", 1)
	voterTwo := gopolls.NewVoter("two", 2)
	voterThree := gopolls.NewVoter("three", 3)

	voteOne := gopolls.NewBasicVote(voterOne, gopolls.Aye)
	voteTwo := gopolls.NewBasicVote(voterTwo, gopolls.Aye)
	voteThree := gopolls.NewBasicVote(voterThree, 42)

	poll := gopolls.NewBasicPoll([]*gopolls.BasicVote{voteOne, voteTwo, voteThree})

	res := poll.Tally()

	expectedVotes := gopolls.BasicPollCounter{
		NumNoes:       0,
		NumAyes:       2,
		NumAbstention: 0,
		NumInvalid:    1,
	}
	if !res.NumberVoters.Equals(&expectedVotes) {
		t.Errorf("Expected basic poll result to be %v, got %v instead",
			expectedVotes, *res.NumberVoters)
	}

	expectedWeightedVotes := gopolls.BasicPollCounter{
		NumNoes:       0,
		NumAyes:       3,
		NumAbstention: 0,
		NumInvalid:    3,
	}
	if !res.WeightedVotes.Equals(&expectedWeightedVotes) {
		t.Errorf("Expected basic poll result to be %v, got %v instead",
			expectedWeightedVotes, *res.WeightedVotes)
	}
}
