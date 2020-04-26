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
	"math"
	"reflect"
	"sort"
	"strconv"
)

// MedianUnit is the unit used in median polls and votes (the value the poll is about).
type MedianUnit uint64

// NoMedianUnitValue is used to signal that a value is not a valid MedianUnit, for example as default argument.
const NoMedianUnitValue MedianUnit = math.MaxUint64

// ParseMedianUnit parses a MedianUnit from a string.
//
// A PollingSyntaxError is returned if s is no valid uint or is NoMedianUnitValue.
func ParseMedianUnit(s string) (MedianUnit, error) {
	asInt, err := strconv.ParseUint(s, 10, 64)
	if err != nil {
		return NoMedianUnitValue, NewPollingSyntaxError(err, "")
	}
	res := MedianUnit(asInt)
	if res == NoMedianUnitValue {
		return NoMedianUnitValue, NewPollingSyntaxError(nil, "integer value %d is too big", NoMedianUnitValue)
	}
	return res, nil
}

// MedianVote is a vote for a MedianPoll.
//
// The vote has a voter (weight taken into account) and the Value the voter voted for.
// It implements the interface AbstractVote.
type MedianVote struct {
	Voter *Voter
	Value MedianUnit
}

// NewMedianVote returns a new median vote given the voter and the value the voter voted for.
func NewMedianVote(voter *Voter, value MedianUnit) *MedianVote {
	return &MedianVote{
		Voter: voter,
		Value: value,
	}
}

// MedianVoteParser implements VoteParser and returns an instance of MedianVote in its ParseFromString method.
//
// It allows a currency value to be parsed.
// The currency value must be > 0, otherwise an error is returned.
//
// The currency is not directly parsed, instead it uses any CurrencyParser, this way the style of the string
// can be adapted to your needs.
//
// It also allows to set a maxValue, that is every vote with a value > maxValue will return an error when parsed.
type MedianVoteParser struct {
	parser   CurrencyParser
	maxValue MedianUnit
}

// NewMedianVoteParser returns a new MedianVoteParser given the currency parser.
//
// The maxValue is set to NoMedianUnitValue, meaning that it is disabled and doesn't check for a max value.
// To enable it use WithMaxValue.
//
// It also implements ParserCustomizer.
func NewMedianVoteParser(currencyParser CurrencyParser) *MedianVoteParser {
	return &MedianVoteParser{
		parser:   currencyParser,
		maxValue: NoMedianUnitValue,
	}
}

// WithMaxValue returns a shallow copy of the parser with only maxValue set to the new value.
func (parser *MedianVoteParser) WithMaxValue(maxValue MedianUnit) *MedianVoteParser {
	return &MedianVoteParser{
		parser:   parser.parser,
		maxValue: maxValue,
	}
}

// CustomizeForPoll implements ParserCustomizer and returns a new parser with maxValue set if a *MedianPoll is given.
func (parser *MedianVoteParser) CustomizeForPoll(poll AbstractPoll) (ParserCustomizer, error) {
	if asMedianPoll, ok := poll.(*MedianPoll); ok {
		return parser.WithMaxValue(asMedianPoll.Value), nil
	}
	return nil, NewPollTypeError("can't customize MedianVoteParser for type %s, expected type *MedianPoll",
		reflect.TypeOf(poll))
}

// ParseFromString implements the VoteParser interface, for details see type description.
func (parser *MedianVoteParser) ParseFromString(s string, voter *Voter) (AbstractVote, error) {
	// try to parse s with the given parser, that's all we need to do
	currency, parseErr := parser.parser.Parse(s)
	if parseErr != nil {
		return nil, NewPollingSyntaxError(parseErr, "error parsing currency")
	}
	// transform into median vote
	if currency.ValueCents < 0 {
		return nil, NewPollingSemanticError(nil, "string %s describes a negative value, can't be used in a median vote", s)
	}
	asMedianUnit := MedianUnit(currency.ValueCents)
	// check if it is in the correct bounds
	if parser.maxValue != NoMedianUnitValue && asMedianUnit > parser.maxValue {
		return nil, NewPollingSemanticError(nil, "value for median vote (%d) is greatre than allowed max value (%d)",
			asMedianUnit, parser.maxValue)
	}
	return NewMedianVote(voter, asMedianUnit), nil
}

// GetVoter returns the voter of the vote.
func (vote *MedianVote) GetVoter() *Voter {
	return vote.Voter
}

// VoteType returns the constant MedianVoteType.
func (vote *MedianVote) VoteType() string {
	return MedianVoteType
}

// MedianPoll is a poll that can be evaluated with the median method. It implements the interface AbstractPoll.
//
// The median method for polls works as follows:
// The value that "wins" the poll is the highest value that has a majority, taking into account the weight of the
// voters. See Tally for details.
//
// Note: If a voter voted for a value > poll.Value this value could be chosen as the winner.
// Because this doesn't make much sense you should take care to "truncate" the votes.
// You can use TruncateVoters for this.
//
// It also has a Sorted attribute which is set to true once the votes are sorted according to value, s.t.
// the highest votes come first. See SortVotes and AssureSorted for this.
// You can set Sorted to true if you have already sorted them (for example in a database query).
// The SortVotes method will in-place sort the Votes, thus changing the original slice.
// The Tally method always calls AssureSorted.
//
// This type also implements VoteGenerator.
type MedianPoll struct {
	Value  MedianUnit
	Votes  []*MedianVote
	Sorted bool
}

// NewMedianPoll returns a new poll given the value in question and the votes for the poll.
// Note: Read the type documentation carefully! This method will set Sorted to False and will not truncate the voters.
func NewMedianPoll(value MedianUnit, votes []*MedianVote) *MedianPoll {
	return &MedianPoll{
		Value:  value,
		Votes:  votes,
		Sorted: false,
	}
}

// PollType returns the constant MedianPollType.
func (poll *MedianPoll) PollType() string {
	return MedianPollType
}

// AddVote adds a vote to the poll, the vote must be of type *MedianVote.
//
// Note that no vote validation is happening here! I.e. the vote can have an "invalid" value, for example a value that
// is too large.
// We do this because in general it is also allowed to append any vote, it is the job of the user of this library
// to deal with invalid votes.
func (poll *MedianPoll) AddVote(vote AbstractVote) error {
	asMedianVote, ok := vote.(*MedianVote)
	if !ok {
		return NewPollTypeError("can't add vote to MedianPoll, vote must be of type *MedianVote, got type %s",
			reflect.TypeOf(vote))
	}
	poll.Votes = append(poll.Votes, asMedianVote)
	return nil
}

// GenerateVoteFromBasicAnswer implements VoteGenerator and returns a MedianVote.
//
// Abstention is not an allowed value here!
// It will return a vote for 0 for No, a vote for poll.Value for Yes.
func (poll *MedianPoll) GenerateVoteFromBasicAnswer(voter *Voter, answer BasicPollAnswer) (AbstractVote, error) {
	switch answer {
	case No:
		return NewMedianVote(voter, 0), nil
	case Aye:
		return NewMedianVote(voter, poll.Value), nil
	case Abstention:
		return nil, NewPollTypeError("abstention is not supported for median polls")
	default:
		return nil, NewPollTypeError("invalid poll answer %d", answer)
	}
}

// TruncateVoters identifies all votes that contain a value > poll.Value.
//
// It could lead to "weird" results if the value the voters agreed upon was > poll.Value.
// This way the poll gets filtered by updating the value of such a vote to poll.Value.
// The result returned contains the original entries with a value > poll.Value (for logging purposes).
// The returned result contains shallow copies of the culprit (i.e. Value is copied and the *Voter object is re-used).
//
// Note: If you use this method the sorting order should be maintained, everyone who voted with a value > poll.Value
// should be at the beginning of the slice and are now set to poll.Value. Because all other votes have a value <=
// poll.Value this should be fine.
// Thus if the votes are already sorted they should be sorted afterwards too.
func (poll *MedianPoll) TruncateVoters() []*MedianVote {
	culprits := make([]*MedianVote, 0)
	for _, vote := range poll.Votes {
		if vote.Value > poll.Value {
			// voted for a too big value ==> truncate to poll.Value and add to "culprit" list
			culprit := NewMedianVote(vote.Voter, vote.Value)
			culprits = append(culprits, culprit)
			vote.Value = poll.Value
		}
	}
	return culprits
}

// SortVotes sorts the votes list in-place according to vote.Value (highest votes first).
func (poll *MedianPoll) SortVotes() {
	// sort votes according to value
	sortFunc := func(i, j int) bool {
		return poll.Votes[i].Value > poll.Votes[j].Value
	}
	sort.SliceStable(poll.Votes, sortFunc)
	poll.Sorted = true
}

// AssureSorted makes sure that the votes are sorted, if they're not sorted (according to poll.Sorted)
// they will be sorted.
func (poll *MedianPoll) AssureSorted() {
	if !poll.Sorted {
		poll.SortVotes()
	}
}

// WeightSum returns the sum of all voters weights.
func (poll *MedianPoll) WeightSum() Weight {
	var sum Weight
	for _, vote := range poll.Votes {
		sum += vote.Voter.Weight
	}
	return sum
}

// MedianResult is the result of evaluating a median poll, see Tally method.
//
// The result contains the following information:
// WeightSum is the sum of all weights from the votes.
// RequiredMajority is the majority that was required for the winning value.
// MajorityValue is the highest value that had the RequiredMajority.
// ValueDetails maps all values that occurred in at least one vote and maps it to the voters that voted for this value.
// This map can be further analyzed with GetVotersForValue.
type MedianResult struct {
	WeightSum        Weight
	RequiredMajority Weight
	MajorityValue    MedianUnit
	ValueDetails     map[MedianUnit][]*Voter
}

// NewMedianResult returns a new MedianResult.
//
// The returned instance has WeightSum and RequiredMajority set to NoWeight, MajorityValue set to NoMedianUnitValue
// and ValueDetails to an empty map.
func NewMedianResult() *MedianResult {
	return &MedianResult{
		WeightSum:        NoWeight,
		RequiredMajority: NoWeight,
		MajorityValue:    NoMedianUnitValue,
		ValueDetails:     make(map[MedianUnit][]*Voter),
	}
}

// addDetail adds a voter to the list of voters for the given value.
func (result *MedianResult) addDetail(value MedianUnit, voter *Voter) {
	votersList, has := result.ValueDetails[value]
	if !has {
		votersList = make([]*Voter, 0)
	}
	votersList = append(votersList, voter)
	result.ValueDetails[value] = votersList
}

// GetVotersForValue can be used to analyze ValueDetails.
//
// Given a referenceValue it returns a list of all voters that voted for a value >= referenceValue.
// Not that the runtime is in O(#voters).
func (result *MedianResult) GetVotersForValue(referenceValue MedianUnit) []*Voter {
	res := make([]*Voter, 0)
	// iterate over all values voted for and add those that voted for a value >= referenceValue
	// could of course be improved with binary trees or whatever, but not so important
	for value, votersList := range result.ValueDetails {
		if value >= referenceValue {
			res = append(res, votersList...)
		}
	}
	return res
}

// Tally computes the result of a median poll.
//
// Majority can be set to the majority that the result requires. It defaults to the sum of all voter weights divided
// by two if set to NoWeight.
// It wins the highest value that can accumulate a weight > (strictly!) majority.
// For computing majorities see ComputeMajority.
//
// An example: If there are 10 voters, each with weight one, the highest value that reaches > 5 (meaning at least 6)
// votes wins.
// If there were 7 such voters > 3 (meaning 4) voters a required.
//
// Note that usually the value 0 should have a majority (because it is the smallest one allowed).
// If there are no voters or majority is incorrect (for example > total weight sum) MajorityValue might be set to
// NoMedianUnitValue.
//
// This method will also make sure that the polls are sorted (AssureSorted).
// The runtime of this method is (for n = number of voters) O(n) if already sorted and O(n * log n) if not sorted.
func (poll *MedianPoll) Tally(majority Weight) *MedianResult {
	poll.AssureSorted()
	weightSum := poll.WeightSum()

	if majority == NoWeight {
		majority = ComputeMajority(FiftyPercentMajority, weightSum)
	}
	res := NewMedianResult()
	res.WeightSum = weightSum
	res.RequiredMajority = majority

	// iterate over the sorted votes and append to the ValueDetails as required

	// currentWeight is the current sum of weights, incremented for each voter
	var currentWeight Weight
	// foundMajority is set to true once a majority has been found
	foundMajority := false

	for _, vote := range poll.Votes {
		// append to details
		res.addDetail(vote.Value, vote.Voter)
		// update weight sum
		currentWeight += vote.Voter.Weight
		// if no majority has been found yet also update the sum and set result variable
		if !foundMajority && currentWeight > majority {
			// found a majority value! set in result and update foundMajority
			res.MajorityValue = vote.Value
			foundMajority = true
		}
	}

	return res
}
