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
	d := poll.ComputeD()
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

	p := poll.ComputeP(d)
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
}

func TestSchulzeWikiTwo(t *testing.T) {
	votes := getSchulzeVotesTesting(4, []gopolls.Weight{3, 2, 2, 2}, 4)
	votes[0].Ranking = gopolls.SchulzeRanking{1, 2, 3, 4}
	votes[1].Ranking = gopolls.SchulzeRanking{2, 3, 4, 1}
	votes[2].Ranking = gopolls.SchulzeRanking{4, 2, 3, 1}
	votes[3].Ranking = gopolls.SchulzeRanking{4, 2, 1, 3}

	poll := gopolls.NewSchulzePoll(4, votes)

	d := poll.ComputeD()
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

	p := poll.ComputeP(d)
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
}
