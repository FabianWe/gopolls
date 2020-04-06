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
	"strings"
)

// BasicPollAnswer is the answer for a poll with the options "No", "Aye / Yes" and "Abstention".
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

// IsValid tests if the answer is valid, i.e. one of the constants No, Aye, Abstention.
func (a BasicPollAnswer) IsValid() bool {
	switch a {
	case No, Aye, Abstention:
		return true
	default:
		return false
	}
}

// BasicVote is a vote for a BasicPoll.
// It is described by the answer and the voter. It implements the interface AbstractVote.
type BasicVote struct {
	Voter  *Voter
	Choice BasicPollAnswer
}

// NewBasicVote returns a new BasicVote.
func NewBasicVote(voter *Voter, choice BasicPollAnswer) *BasicVote {
	return &BasicVote{
		Voter:  voter,
		Choice: choice,
	}
}

// BasicVoteParser implements VoteParser and returns an instance of BasicVote in its ParseFromString method.
//
// It allows two styles of strings:
//
// First a simple string that describes No, Aye/Yes and Abstention.
// The lists of valid strings for these options can be configured with the NoValues, AyeValues and AbstentionValues
// lists.
// These lists must be all lower case strings defining the valid options.
// The defaults, as created by NewBasicVoteParser, are (English and German words):
// {"+", "n", "no", "nein", "dagegen"} for NoValues,
// {"-", "a", "aye", "y", "yes", "ja", "dafür"} for AyeValues and
// {"/", "abstention", "enthaltung"} for AbstentionValues.
//
// The second style is in the form of Schulze vote, i.e. two integers in the form "a, b". These two are the ranking
// positions if this vote would be interpreted as in a Schulze poll (see Schulze method for details).
// The number a is the sorting position for the Yes/Aye option, b the sorting position for the No option.
// Thus a string "a, b" (for valid integers a and b) is translated to:
// Aye if a < b, No if b < a and Abstention if a = b.
//
// Because this style might be confusing for people not familiar with the Schulze method the acceptance of the ranking
// style can be disabled with AllowRankingStyle = false,
type BasicVoteParser struct {
	NoValues          []string
	AyeValues         []string
	AbstentionValues  []string
	AllowRankingStyle bool
}

// NewBasicVoteParser returns a new BasicVoteParser with the default strings as described in the type description
// and AllowRankingStyle set to true.
func NewBasicVoteParser() *BasicVoteParser {
	noDefaults := []string{"+", "n", "no", "nein", "dagegen"}
	ayeDefaults := []string{"-", "a", "aye", "y", "yes", "ja", "dafür"}
	abstentionDefaults := []string{"/", "abstention", "enthaltung"}
	return &BasicVoteParser{
		NoValues:          noDefaults,
		AyeValues:         ayeDefaults,
		AbstentionValues:  abstentionDefaults,
		AllowRankingStyle: true,
	}
}

func (parser *BasicVoteParser) containsString(candidates []string, s string) bool {
	s = strings.ToLower(s)
	for _, candidate := range candidates {
		if candidate == s {
			return true
		}
	}
	return false
}

func (parser *BasicVoteParser) basicStyle(s string, voter *Voter) (*BasicVote, bool) {
	var answer BasicPollAnswer = -1
	switch {
	case parser.containsString(parser.NoValues, s):
		answer = No
	case parser.containsString(parser.AyeValues, s):
		answer = Aye
	case parser.containsString(parser.AbstentionValues, s):
		answer = Abstention
	}
	if answer < 0 {
		return nil, false
	} else {
		return NewBasicVote(voter, answer), true
	}
}

func (parser *BasicVoteParser) rankingStyle(s string, voter *Voter) (*BasicVote, bool) {
	ranking, rankingErr := parserSchulzeRanking(s, 2)
	if rankingErr != nil {
		return nil, false
	}
	// now we have a valid ranking, find out what it means
	ayeNum, noNum := ranking[0], ranking[1]
	var answer = Abstention
	switch {
	case ayeNum < noNum:
		answer = Aye
	case ayeNum > noNum:
		answer = No
	}
	return NewBasicVote(voter, answer), true
}

// ParseFromString implements the VoteParser interface, for details see type description.
func (parser *BasicVoteParser) ParseFromString(s string, voter *Voter) (AbstractVote, error) {
	// first try the "default" style with no, yes etc.
	var vote *BasicVote
	var ok bool

	vote, ok = parser.basicStyle(s, voter)
	if ok {
		return vote, nil
	}

	allowedNoString := strings.Join(parser.NoValues, ", ")
	allowedAyeString := strings.Join(parser.AyeValues, ", ")
	allowedAbstentionString := strings.Join(parser.AbstentionValues, ", ")

	// try ranking style, but only if this is allowed
	if !parser.AllowRankingStyle {
		return nil,
			NewPollingSyntaxError(nil, "invalid option (\"%s\") for basic vote (\"%s\"), allowed are: no: \"%s\", aye: \"%s\", abstention",
				s, allowedNoString, allowedAyeString, allowedAbstentionString)
	}
	vote, ok = parser.rankingStyle(s, voter)
	if ok {
		return vote, nil
	}

	// no style matched ==> error
	err := NewPollingSyntaxError(nil, "invalid option (\"%s\") for basic vote , allowed are: no: \"%s\", aye: \"%s\", abstention: \"%s\" or ranking style",
		s, allowedNoString, allowedAyeString, allowedAbstentionString)
	return nil, err
}

// GetVoter returns the voter of the vote.
func (vote *BasicVote) GetVoter() *Voter {
	return vote.Voter
}

// VoteType returns the constant BasicVoteType.
func (vote *BasicVote) VoteType() string {
	return BasicVoteType
}

// BasicPoll is a poll with the options No, Yes and Abstention, for details see BasicPollAnswer.
// It implements the interface AbstractPoll.
type BasicPoll struct {
	Votes []*BasicVote
}

// NewBasicPoll returns a new BasicPoll with the given votes.
func NewBasicPoll(votes []*BasicVote) *BasicPoll {
	return &BasicPoll{votes}
}

// PollType returns the constant BasicPollType.
func (poll *BasicPoll) PollType() string {
	return BasicPollType
}

// TruncateVoters is one of the truncate methods that exist for nearly every poll (really for everyone implemented
// directly in this library).
//
// It finds votes that are "invalid".
// By invalid we mean votes whose answer is none of the constants No, Aye or Abstention.
//
// In general instead of using this method it would probably be easier to evaluate the poll with Tally
// and just look if there are invalid votes in the result.
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

// BasicPollCounter is used to count how often a certain option was chosen.
type BasicPollCounter struct {
	NumNoes, NumAyes, NumAbstention, NumInvalid Weight
}

// NewBasicPollCounter returns a new BasicPollCounter with all counters set to 0.
func NewBasicPollCounter() *BasicPollCounter {
	return &BasicPollCounter{}
}

// Increase increases the counter given the choice, the counter increased depends on choice.
// inc is the value by which the counter is increased.
func (counter *BasicPollCounter) Increase(choice BasicPollAnswer, inc Weight) {
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

// Equals tests if two counter objects store the same state.
func (counter *BasicPollCounter) Equals(other *BasicPollCounter) bool {
	return counter.NumNoes == other.NumNoes &&
		counter.NumAyes == other.NumAyes &&
		counter.NumAbstention == other.NumAbstention &&
		counter.NumInvalid == other.NumInvalid
}

// BasicPollResult is the result of evaluating a BasicPoll.
//
// It stores two instances of BasicPollCounter: NumberVoters counts how often an answer was taken, independent
// of the weight of the voter.
// WeightedVotes counts how often an answer was taken, by summing up not the number of voters but the weight of
// these voters.
type BasicPollResult struct {
	NumberVoters  *BasicPollCounter
	WeightedVotes *BasicPollCounter
}

// NewBasicPollResult returns a new BasicPollResult with all values set to 0.
func NewBasicPollResult() *BasicPollResult {
	return &BasicPollResult{
		NumberVoters:  NewBasicPollCounter(),
		WeightedVotes: NewBasicPollCounter(),
	}
}

func (res *BasicPollResult) increaseCounters(vote *BasicVote) {
	res.NumberVoters.Increase(vote.Choice, 1)
	res.WeightedVotes.Increase(vote.Choice, vote.Voter.Weight)
}

// Tally counts how often a certain answer was taken.
// Note that invalid votes might occur and will be counted in the NumInvalid fields.
func (poll *BasicPoll) Tally() *BasicPollResult {
	res := NewBasicPollResult()
	for _, vote := range poll.Votes {
		res.increaseCounters(vote)
	}
	return res
}
