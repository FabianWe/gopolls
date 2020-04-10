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
	"fmt"
	"github.com/FabianWe/gopolls"
	"testing"
)

func getSchulzeVotesTesting(voters int, weights []gopolls.Weight, numOptions int) []*gopolls.SchulzeVote {
	res := make([]*gopolls.SchulzeVote, 0, voters)

	for i := 0; i < voters; i++ {
		voter := gopolls.NewVoter(fmt.Sprintf("Voter %d", i), weights[i])
		ranking := make(gopolls.SchulzeRanking, numOptions)
		vote := gopolls.NewSchulzeVote(voter, ranking)
		res = append(res, vote)
	}

	return res
}

func compareCandidateGroup(a, b []int) bool {
	// ignore order ==> create set
	ma := make(map[int]bool, len(a))
	mb := make(map[int]bool, len(b))

	for _, value := range a {
		ma[value] = true
	}

	for _, value := range b {
		mb[value] = true
	}

	if len(ma) != len(mb) {
		return false
	}

	for valOne := range ma {
		if _, has := mb[valOne]; !has {
			return false
		}
	}
	return true
}

func compareWeightLists(a, b []gopolls.Weight) bool {
	if len(a) != len(b) {
		return false
	}
	for i, entry := range a {
		if entry != b[i] {
			return false
		}
	}
	return true
}

func TestSchulzeWikiOne(t *testing.T) {
	// first create all votes, there are 8
	votes := getSchulzeVotesTesting(8, []gopolls.Weight{5, 5, 8, 3, 7, 2, 7, 8}, 5)
	votes[0].Ranking = gopolls.SchulzeRanking{1, 3, 2, 5, 4}
	votes[1].Ranking = gopolls.SchulzeRanking{1, 5, 4, 2, 3}
	votes[2].Ranking = gopolls.SchulzeRanking{4, 1, 5, 3, 2}
	votes[3].Ranking = gopolls.SchulzeRanking{2, 3, 1, 5, 4}
	votes[4].Ranking = gopolls.SchulzeRanking{2, 4, 1, 5, 3}
	votes[5].Ranking = gopolls.SchulzeRanking{3, 2, 1, 4, 5}
	votes[6].Ranking = gopolls.SchulzeRanking{5, 4, 2, 1, 3}
	votes[7].Ranking = gopolls.SchulzeRanking{3, 2, 5, 4, 1}

	poll := gopolls.NewSchulzePoll(5, votes)
	res := poll.Tally()
	d := res.D
	expectedD := gopolls.SchulzeMatrix{
		{0, 20, 26, 30, 22},
		{25, 0, 16, 33, 18},
		{19, 29, 0, 17, 24},
		{15, 12, 28, 0, 14},
		{23, 27, 21, 31, 0},
	}
	if !expectedD.Equals(d) {
		t.Errorf("Expected matrix d to be %v, but got %v instead", expectedD, d)
		return
	}

	p := res.P
	expectedP := gopolls.SchulzeMatrix{
		{0, 28, 28, 30, 24},
		{25, 0, 28, 33, 24},
		{25, 29, 0, 29, 24},
		{25, 28, 28, 0, 24},
		{25, 28, 28, 31, 0},
	}

	if !expectedP.Equals(p) {
		t.Errorf("Expected matrix p to be %v, but got %v instead", expectedP, p)
		return
	}

	ranking := res.RankedGroups
	if len(ranking) != 5 {
		t.Errorf("Expected ranked matrix of p to contain 5 groups, got %v instead", ranking)
		return
	}
	rankingTests := []struct {
		gotGroup, expectedGroup []int
	}{
		{
			ranking[0],
			[]int{4},
		},
		{
			ranking[1],
			[]int{0},
		},
		{
			ranking[2],
			[]int{2},
		},
		{
			ranking[3],
			[]int{1},
		},
		{
			ranking[4],
			[]int{3},
		},
	}
	for i, tc := range rankingTests {
		if !compareCandidateGroup(tc.expectedGroup, tc.gotGroup) {
			t.Errorf("In rankP: Expected in group %d the following list of options: %v. got %v instead",
				i, tc.expectedGroup, tc.gotGroup)
		}
	}
}

func TestSchulzeWikiTwo(t *testing.T) {
	votes := getSchulzeVotesTesting(4, []gopolls.Weight{3, 2, 2, 2}, 4)
	votes[0].Ranking = gopolls.SchulzeRanking{1, 2, 3, 4}
	votes[1].Ranking = gopolls.SchulzeRanking{2, 3, 4, 1}
	votes[2].Ranking = gopolls.SchulzeRanking{4, 2, 3, 1}
	votes[3].Ranking = gopolls.SchulzeRanking{4, 2, 1, 3}

	poll := gopolls.NewSchulzePoll(4, votes)
	res := poll.Tally()

	d := res.D
	expectedD := gopolls.SchulzeMatrix{
		{0, 5, 5, 3},
		{4, 0, 7, 5},
		{4, 2, 0, 5},
		{6, 4, 4, 0},
	}
	if !expectedD.Equals(d) {
		t.Errorf("Expected matrix d to be %v, but got %v instead", expectedD, d)
		return
	}

	p := res.P
	expectedP := gopolls.SchulzeMatrix{
		{0, 5, 5, 5},
		{5, 0, 7, 5},
		{5, 5, 0, 5},
		{6, 5, 5, 0},
	}
	if !expectedP.Equals(p) {
		t.Errorf("Expected matrix p to be %v, but got %v instead", expectedP, p)
		return
	}
	// winner should be second and fourth option (B & D in the example)
	ranking := res.RankedGroups
	groupOne := ranking[0]
	expectedGroupOne := []int{1, 3}
	if !compareCandidateGroup(groupOne, expectedGroupOne) {
		t.Errorf("Expected ranking of matrix p to be %v, but got %v instead", expectedGroupOne, groupOne)
	}
}

func TestSmallComputeD(t *testing.T) {
	// just a very small test that d (and non strict d) are computed as one would expect
	votes := getSchulzeVotesTesting(5, []gopolls.Weight{1, 2, 3, 4, 5}, 3)
	votes[0].Ranking = gopolls.SchulzeRanking{1, 0, 1}
	votes[1].Ranking = gopolls.SchulzeRanking{0, 1, 0}
	votes[2].Ranking = gopolls.SchulzeRanking{0, 0, 0}
	votes[3].Ranking = gopolls.SchulzeRanking{1, 1, 0}
	votes[4].Ranking = gopolls.SchulzeRanking{1, 2, 3}
	poll := gopolls.NewSchulzePoll(3, votes)
	res := poll.Tally()

	expectedD := gopolls.SchulzeMatrix{
		{0, 7, 5},
		{1, 0, 6},
		{4, 6, 0},
	}
	if !expectedD.Equals(res.D) {
		t.Errorf("Expected matrix d to be %v, but got %v instead", expectedD, res.D)
	}

	expectedDNonStrict := gopolls.SchulzeMatrix{
		{0, 14, 11},
		{8, 0, 9},
		{10, 9, 0},
	}
	if !expectedDNonStrict.Equals(res.DNonStrict) {
		t.Errorf("Expected matrix d (non-strict) to be %v, but got %v instead", expectedDNonStrict, res.DNonStrict)
	}
	// also test that better than no are correct
	expectedBetterThanNo := []gopolls.Weight{5, 6, 0}
	betterThanNo := res.StrictlyBetterThanNo()
	if !compareWeightLists(expectedBetterThanNo, betterThanNo) {
		t.Errorf("Expected strictly better than no list to be %v, but got %v instead", expectedBetterThanNo, betterThanNo)
	}

	expectedBetterOrEqualNo := []gopolls.Weight{11, 9, 0}
	betterOrEqualNo := res.BetterOrEqualNo()
	if !compareWeightLists(expectedBetterOrEqualNo, betterOrEqualNo) {
		t.Errorf("Expected better or equal than no list to be %v, but got %v instead", expectedBetterOrEqualNo, betterOrEqualNo)
	}
}
