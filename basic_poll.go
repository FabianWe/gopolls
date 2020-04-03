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

type BasicPollAnswer int8

const (
	No BasicPollAnswer = iota
	Aye
	Abstention
)

func (a BasicPollAnswer) String() string {
	switch a {
	case No:
		return "no"
	case Aye:
		return "aye"
	case Abstention:
		return "abstention"
	default:
		return fmt.Sprintf("Unkown poll answer %d", a)
	}
}

func (a BasicPollAnswer) IsValid() bool {
	switch a {
	case No, Aye, Abstention:
		return true
	default:
		return false
	}
}

type BasicVote struct {
	Voter  *Voter
	Choice BasicPollAnswer
}

func NewBasicVote(voter *Voter, choice BasicPollAnswer) *BasicVote {
	return &BasicVote{
		Voter:  voter,
		Choice: choice,
	}
}

type BasicPoll struct {
	Votes []*BasicVote
}

func NewBasicPoll(votes []*BasicVote) *BasicPoll {
	return &BasicPoll{votes}
}

func (poll *BasicPoll) TruncateVoters() []*BasicVote {
	// culprits: all with an invalid choice
	// filtered: the filtered list to use as new votes
	// to avoid creating the filtered list we compute filtered only if we know there are culprits
	// usually there should be no culprits and we want to avoid to copy everything in this case

	culprits := make([]*BasicVote, 0)
	filtered := poll.Votes

	for _, vote := range poll.Votes {
		if !vote.Choice.IsValid() {
			culprits = append(culprits, vote)
		}
	}

	// now only if we found culprits we create a new filtered list
	if len(culprits) > 0 {
		filtered = make([]*BasicVote, 0, len(poll.Votes)-len(culprits))
		// same loop as above again, but this time not to add culprits but to add the valid ones
		for _, vote := range poll.Votes {
			if vote.Choice.IsValid() {
				filtered = append(filtered, vote)
			}
		}
	}

	poll.Votes = filtered
	return culprits
}

type BasicPollCounter struct {
	NumNoes, NumAyes, NumAbstention, NumInvalid Weight
}

func NewBasicPollCounter() *BasicPollCounter {
	return &BasicPollCounter{}
}

func (counter *BasicPollCounter) increase(choice BasicPollAnswer, inc Weight) {
	switch choice {
	case No:
		counter.NumNoes += inc
	case Aye:
		counter.NumAyes += inc
	case Abstention:
		counter.NumAbstention += inc
	default:
		counter.NumInvalid += inc
	}
}

func (counter *BasicPollCounter) Equals(other *BasicPollCounter) bool {
	return counter.NumNoes == other.NumNoes &&
		counter.NumAyes == other.NumAyes &&
		counter.NumAbstention == other.NumAbstention &&
		counter.NumInvalid == other.NumInvalid
}

type BasicPollResult struct {
	NumberVoters  *BasicPollCounter
	WeightedVotes *BasicPollCounter
}

func NewBasicPollResult() *BasicPollResult {
	return &BasicPollResult{
		NumberVoters:  NewBasicPollCounter(),
		WeightedVotes: NewBasicPollCounter(),
	}
}

func (res *BasicPollResult) increaseCounters(vote *BasicVote) {
	res.NumberVoters.increase(vote.Choice, 1)
	res.WeightedVotes.increase(vote.Choice, vote.Voter.Weight)
}

func (poll *BasicPoll) Tally() *BasicPollResult {
	res := NewBasicPollResult()
	for _, vote := range poll.Votes {
		res.increaseCounters(vote)
	}
	return res
}
