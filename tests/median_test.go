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

func assertDetails(t *testing.T, forValue gopolls.MedianUnit, expected, got []*gopolls.Voter) {
	// for easier testing we create a map from voter name to voter object
	expectedMap := make(map[string]*gopolls.Voter, len(expected))
	for _, voter := range expected {
		expectedMap[voter.Name] = voter
	}
	gotMap := make(map[string]*gopolls.Voter, len(got))
	for _, voter := range got {
		gotMap[voter.Name] = voter
	}
	if len(expectedMap) != len(gotMap) {
		t.Errorf("Expected median details for value %v to be %v, but got %v instead",
			forValue, expected, got)
		return
	}
	for name, expectedVoter := range expectedMap {
		gotVoter, has := gotMap[name]
		if !has {
			t.Errorf("Expected name \"%s\" to appear in result, but didn't find it", name)
			return
		}
		if !expectedVoter.Equals(gotVoter) {
			t.Errorf("Expected median details for value %v to be %v, but got %v instead",
				forValue, expected, got)
			return
		}
	}
}

func TestMedianOne(t *testing.T) {
	voterOne := gopolls.NewVoter("one", 4)
	voterTwo := gopolls.NewVoter("two", 3)
	voterThree := gopolls.NewVoter("three", 2)
	voterFour := gopolls.NewVoter("four", 2)

	voteOne := gopolls.NewMedianVote(voterOne, 200)
	voteTwo := gopolls.NewMedianVote(voterTwo, 1000)
	voteThree := gopolls.NewMedianVote(voterThree, 700)
	voteFour := gopolls.NewMedianVote(voterFour, 500)

	poll := gopolls.NewMedianPoll(1000, []*gopolls.MedianVote{voteOne, voteTwo, voteThree, voteFour})

	res := poll.Tally(gopolls.NoWeight)

	if res.WeightSum != 11 {
		t.Errorf("Expected weight sum to be 11, got %d instead", res.WeightSum)
	}

	if res.RequiredMajority != 5 {
		t.Errorf("Expected majority to be 5, got %d instead", res.RequiredMajority)
	}

	if res.MajorityValue != 500 {
		t.Errorf("Expected majority value to be 500, got %d instead", res.MajorityValue)
	}
	// also test details
	assertDetails(t, 1000, []*gopolls.Voter{voterTwo}, res.GetVotersForValue(1000))
	assertDetails(t, 500, []*gopolls.Voter{voterTwo, voterThree, voterFour}, res.GetVotersForValue(500))
	assertDetails(t, 501, []*gopolls.Voter{voterTwo, voterThree}, res.GetVotersForValue(501))
	assertDetails(t, 0, []*gopolls.Voter{voterOne, voterTwo, voterThree, voterFour}, res.GetVotersForValue(0))
}

func TestMedianTwo(t *testing.T) {
	voterOne := gopolls.NewVoter("one", 1)
	voterTwo := gopolls.NewVoter("two", 2)
	voterThree := gopolls.NewVoter("three", 3)

	voteOne := gopolls.NewMedianVote(voterOne, 0)
	voteTwo := gopolls.NewMedianVote(voterTwo, 150)
	voteThree := gopolls.NewMedianVote(voterThree, 200)

	poll := gopolls.NewMedianPoll(1000, []*gopolls.MedianVote{voteOne, voteTwo, voteThree})

	res := poll.Tally(gopolls.NoWeight)

	if res.WeightSum != 6 {
		t.Errorf("Expected weight sum to be 11, got %d instead", res.WeightSum)
	}

	if res.RequiredMajority != 3 {
		t.Errorf("Expected majority to be 5, got %d instead", res.RequiredMajority)
	}

	if res.MajorityValue != 150 {
		t.Errorf("Expected majority value to be 500, got %d instead", res.MajorityValue)
	}
	// not so many detail tests as before
	assertDetails(t, 149, []*gopolls.Voter{voterTwo, voterThree}, res.GetVotersForValue(149))
}

func TestMedianTruncateVoters(t *testing.T) {
	voterOne := gopolls.NewVoter("one", 1)
	voterTwo := gopolls.NewVoter("two", 2)
	voterThree := gopolls.NewVoter("three", 3)

	voteOne := gopolls.NewMedianVote(voterOne, 200)
	voteTwo := gopolls.NewMedianVote(voterTwo, 150)
	voteThree := gopolls.NewMedianVote(voterThree, 100)

	poll := gopolls.NewMedianPoll(150, []*gopolls.MedianVote{voteOne, voteTwo, voteThree})
	poll.Sorted = true
	truncated := poll.TruncateVoters()
	if len(truncated) != 1 {
		t.Errorf("Expected one truncated vote, but got %v instead", truncated)
		return
	}
	if truncated[0].Voter.Name != "one" {
		t.Errorf("Voter \"one\" should have been truncated, got name \"%s\" instead", truncated[0].Voter.Name)
		return
	}
	if poll.Votes[0].Voter.Name != "one" {
		t.Errorf("Voter \"one\" should still be first in list, got name \"%s\" instead", truncated[0].Voter.Name)
		return
	}
	if poll.Votes[0].Value != 150 {
		t.Errorf("Vote for \"one\" should have been truncated to 150, got %d instead", poll.Votes[0].Value)
	}
}
