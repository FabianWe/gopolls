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
	"sort"
	"strconv"
	"strings"
)

// SchulzeMatrix is a matrix used to represent the matrices d and p.
// It is assumed to be of dimension n × n.
type SchulzeMatrix [][]Weight

// NewSchulzeMatrix returns a new matrix given the dimension, so the resulting matrix is of size n × n.
func NewSchulzeMatrix(dimension int) SchulzeMatrix {
	var res SchulzeMatrix = make(SchulzeMatrix, dimension)
	for i := 0; i < dimension; i++ {
		res[i] = make([]Weight, dimension)
	}
	return res
}

// Equals tests if two matrices are the same.
// Note that this method (like all others) assume a matrix of size n × n.
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

// SchulzeRanking is a ranking for a Schulze poll.
//
// The ranking must have one entry for each option of the poll.
// The entries of the ranking describe the ranked position for the option.
//
// Consider a poll with three alternatives ["A", "B", "C"].
// Then the ranking [1, 0, 1] would represent the ranking B > A = C.
// That is the smaller the value the more "important" or higher ranked the option.
type SchulzeRanking []int

// NewSchulzeRanking returns a new SchulzeRanking with a size of 0.
func NewSchulzeRanking() SchulzeRanking {
	return make(SchulzeRanking, 0)
}

// private because from outside the parser implementing the parser interface should be used
func parserSchulzeRanking(s string, length int) (SchulzeRanking, error) {
	split := strings.FieldsFunc(s, func(r rune) bool {
		return r == ',' || r == '/'
	})
	if length >= 0 && len(split) != length {
		return nil, NewPollingSyntaxError(nil, "schulze ranking of length %d was expected, got length %d",
			length, len(split))
	}
	res := make(SchulzeRanking, len(split))
	for i, asString := range split {
		asString = strings.TrimSpace(asString)
		asInt, intErr := strconv.Atoi(asString)
		if intErr != nil {
			return nil, NewPollingSyntaxError(intErr, "can't parse schulze ranking")
		}
		res[i] = asInt
	}
	return res, nil
}

// SchulzeVote is a vote for a SchulzePoll.
// It is described by the voter and the ranking of said voter. It implements the interface AbstractVote.
type SchulzeVote struct {
	Voter   *Voter
	Ranking SchulzeRanking
}

// NewSchulzeVote returns a new SchulzeVote.
func NewSchulzeVote(voter *Voter, ranking SchulzeRanking) *SchulzeVote {
	return &SchulzeVote{
		Voter:   voter,
		Ranking: ranking,
	}
}

// SchulzeVoteParser implements VoteParser and returns an instance of SchulzeVote in its ParseFromString method.
//
// The ranking is assumed to be a comma separated list of integers, for example "1, 0, 1" (slashes are also okay,
// so "1/0/1" would be the same).
// See documentation of SchulzeRanking for more details about the ranking.
//
// It allows to set the length that is expected from the ranking string. If the string describes a ranking
// not equal to length an error is returned.
type SchulzeVoteParser struct {
	Length int
}

// NewSchuleVoteParser returns a new SchulzeVoteParser.
//
// The length argument is allowed to be negative in which case the length check is disabled.
// Set it to a length >= 0 to enable the check or use WithLength.
func NewSchuleVoteParser(length int) *SchulzeVoteParser {
	return &SchulzeVoteParser{Length: length}
}

// WithLength returns a shallow copy of the parser with only length set to the new value.
func (parser *SchulzeVoteParser) WithLength(length int) *SchulzeVoteParser {
	return &SchulzeVoteParser{Length: length}
}

// ParseFromString implements the VoteParser interface, for details see type description.
func (parser *SchulzeVoteParser) ParseFromString(s string, voter *Voter) (AbstractVote, error) {
	ranking, err := parserSchulzeRanking(s, parser.Length)
	if err != nil {
		return nil, err
	}
	return NewSchulzeVote(voter, ranking), nil
}

// GetVoter returns the voter of the vote.
func (vote *SchulzeVote) GetVoter() *Voter {
	return vote.Voter
}

// VoteType returns the constant SchulzeVoteType.
func (vote *SchulzeVote) VoteType() string {
	return SchulzeVoteType
}

// SchulzeWinsList describes the winning groups of a Schulze poll.
// The first list contains all options  that are ranked highest, the next list all entries ranked second
// best and so on.
// Each option should appear in at least one of the lists.
type SchulzeWinsList [][]int

// SchulzePoll is a poll that can be evaluated with the Schulze method, see https://en.wikipedia.org/wiki/Schulze_method
// for details.
// It implements the interface AbstractPoll.
//
// A poll instance has the number of options in the poll (must be a positive int) and all votes for the poll.
//
// Note that all votes must have a ranking of length NumVotes. If this is not the case the the vote
// will be silently dropped. You should use TruncateVoters first to identify problematic cases.
//
// The implementation was inspired by the German Wikipedia article (https://de.wikipedia.org/wiki/Schulze-Methode)
// and https://github.com/mgp/schulze-method.
type SchulzePoll struct {
	NumOptions int
	Votes      []*SchulzeVote
}

// NewSchulzePoll returns a new SchulzePoll.
// numOptions must be >= 0, otherwise this function panics.
// Note that the votes are not validated (have the correct ranking length).
// Use TruncateVoters to identify invalid votes.
func NewSchulzePoll(numOptions int, votes []*SchulzeVote) *SchulzePoll {
	if numOptions < 0 {
		panic(fmt.Sprintf("Num options in SchulzePoll must be >= 0, got %d", numOptions))
	}
	return &SchulzePoll{
		NumOptions: numOptions,
		Votes:      votes,
	}
}

// PollType returns the constant SchulzePollType.
func (poll *SchulzePoll) PollType() string {
	return SchulzePollType
}

// TruncateVoters removes all voters that have a ranking with length != poll.NumOptions.
//
// If such culprits are found they are removed from poll.Votes. In this case a new slice of votes
// will be allocated containing the original vote objects.
// All culprits are returned (for logging or error handling).
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
		if len(ranking) != n {
			continue
		}
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

// inspired by https://github.com/mgp/schulze-method/blob/master/schulze.py
func (poll *SchulzePoll) rankP(p SchulzeMatrix) SchulzeWinsList {
	n := poll.NumOptions
	// maps: number of wins to candidates with numwins
	candidateWins := make(map[uint64][]int)
	numWinsKeys := make([]uint64, 0)
	for i := 0; i < n; i++ {
		var numWins uint64
		for j := 0; j < n; j++ {
			if i != j && p[i][j] > p[j][i] {
				numWins++
			}
		}
		winsList, has := candidateWins[numWins]
		if !has {
			winsList = make([]int, 0, 1)
			numWinsKeys = append(numWinsKeys, numWins)
		}
		winsList = append(winsList, i)
		candidateWins[numWins] = winsList
	}
	// now sort the keys according to the one that wins most
	cmp := func(i, j int) bool {
		return numWinsKeys[i] > numWinsKeys[j]
	}
	sort.Slice(numWinsKeys, cmp)
	// now create result list, use sorted keys for order
	res := make(SchulzeWinsList, 0, len(numWinsKeys))
	for _, key := range numWinsKeys {
		res = append(res, candidateWins[key])
	}
	return res
}

// SchulzeResult is the result returned by the Schulze method.
//
// It stores (for testing and further investigation) the matrices d and p and of course the
// sorted winning groups as a SchulzeWinsList.
type SchulzeResult struct {
	D, P         SchulzeMatrix
	RankedGroups SchulzeWinsList
}

// NewSchulzeResult returns a new SchulzeResult.
func NewSchulzeResult(d, p SchulzeMatrix, rankedGroups SchulzeWinsList) *SchulzeResult {
	return &SchulzeResult{
		D:            d,
		P:            p,
		RankedGroups: rankedGroups,
	}
}

// Tally computes the result of a Schulze poll.
//
// Note that all voters with an invalid ranking (length is not poll.NumOptions) are silently discarded.
// Use TruncateVoters before to find such votes.
func (poll *SchulzePoll) Tally() *SchulzeResult {
	d := poll.computeD()
	p := poll.computeP(d)
	rankedGroups := poll.rankP(p)
	return NewSchulzeResult(d, p, rankedGroups)
}
