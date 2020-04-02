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

import "fmt"

type SchulzeMatrix [][]Weight

func NewSchulzeMatrix(dimension int) SchulzeMatrix {
	var res SchulzeMatrix = make(SchulzeMatrix, dimension)
	for i := 0; i < dimension; i++ {
		res[i] = make([]Weight, dimension)
	}
	return res
}

func (m SchulzeMatrix) Equals(other SchulzeMatrix) bool {
	n1, n2 := len(m), len(other)
	if n1 != n2 {
		return false
	}
	n := n1
	for i := 0; i < n; i++ {
		row1, row2 := m[i], other[i]
		for j := 0; j < n; j++ {
			if row1[j] != row2[j] {
				return false
			}
		}
	}
	return true
}

type SchulzeRanking []int

type SchulzeVote struct {
	Voter   *Voter
	Ranking SchulzeRanking
}

func NewSchulzeVote(voter *Voter, ranking SchulzeRanking) *SchulzeVote {
	return &SchulzeVote{
		Voter:   voter,
		Ranking: ranking,
	}
}

type SchulzePoll struct {
	NumOptions int
	Votes      []*SchulzeVote
}

func NewSchulzePoll(numOptions int, votes []*SchulzeVote) *SchulzePoll {
	if numOptions < 0 {
		panic(fmt.Sprintf("Num options in SchulzePoll must be >= 0, got %d", numOptions))
	}
	return &SchulzePoll{
		NumOptions: numOptions,
		Votes:      votes,
	}
}

func (poll *SchulzePoll) TruncateVoters() []*SchulzeVote {
	// culprits: all with an invalid number of elements in ranking
	// filtered: the filtered list to use as new votes
	// to avoid creating the filtered list we compute filtered only if we know there are culprits
	// usually there should be no culprits and we want to avoid to copy everything in this case
	culprits := make([]*SchulzeVote, 0)
	filtered := poll.Votes

	for _, vote := range poll.Votes {
		if poll.NumOptions != len(vote.Ranking) {
			culprits = append(culprits, vote)
		}
	}

	// now only if we found culprits we create a new filtered list
	if len(culprits) > 0 {
		filtered = make([]*SchulzeVote, 0, len(poll.Votes)-len(culprits))
		// same loop as above again, but this time not to add culprits but to add the valid ones
		for _, vote := range poll.Votes {
			if poll.NumOptions == len(vote.Ranking) {
				filtered = append(filtered, vote)
			}
		}
	}

	poll.Votes = filtered
	return culprits
}

func (poll *SchulzePoll) computeD() SchulzeMatrix {
	n := poll.NumOptions
	res := NewSchulzeMatrix(n)

	for _, vote := range poll.Votes {
		w := vote.Voter.Weight
		ranking := vote.Ranking
		for i := 0; i < n; i++ {
			for j := i + 1; j < n; j++ {
				if ranking[i] < ranking[j] {
					res[i][j] += w
				} else if ranking[j] < ranking[i] {
					res[j][i] += w
				}
			}
		}
	}

	return res
}

func (poll *SchulzePoll) computeP(d SchulzeMatrix) SchulzeMatrix {
	n := poll.NumOptions
	res := NewSchulzeMatrix(n)

	for i := 0; i < n; i++ {
		for j := 0; j < n; j++ {
			if i != j && d[i][j] > d[j][i] {
				res[i][j] = d[i][j]
			}
		}
	}

	for i := 0; i < n; i++ {
		for j := 0; j < n; j++ {
			if i != j {
				for k := 0; k < n; k++ {
					if i != k && j != k {
						res[j][k] = WeightMax(res[j][k], WeightMin(res[j][i], res[i][k]))
					}
				}
			}
		}
	}

	return res
}
