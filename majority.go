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
	"math/big"
)

var (
	FiftyPercentMajority = big.NewRat(1, 2)
	TwoThirdsMajority    = big.NewRat(2, 3)
)

// ComputeMajority computes the required majority given the majority as a rational.
// The rational majority must be a value <= 1, for example 1/2 for 50 percent or 2/3 for two thirds majority, see
// also the constants FiftyPercentMajority and TwoThirdsMajority.
//
// For example consider that there are 10 votes (or sum of weights). Then ComputeMajority(1/2, 10) returns 5,
// meaning that > 5 (strictly greater!) votes are required.
// ComputeMajority(2/3, 10) would return 6, meaning that > 6 votes are required.
func ComputeMajority(majority *big.Rat, votesSum Weight) Weight {
	majorityFraction := big.NewRat(int64(votesSum), 1)
	// multiply with requiredMajority
	majorityFraction.Mul(majorityFraction, majority)
	// divide num // denom, this gives use the majority required (i.e. we just drop everything after .)
	// example: 10/2 ==> 5/1 ==> required majority is > 5
	num := majorityFraction.Num()
	denom := majorityFraction.Denom()
	div := new(big.Int)
	div.Div(num, denom)
	asInt := div.Int64()
	// majority <= 1 ==> should be possible to represent as uint32 (Weight)
	return Weight(asInt)
}

func ComputePercentage(votes, votesSum Weight) *big.Rat {
	if votesSum == 0 {
		return big.NewRat(0, 1)
	}
	return big.NewRat(int64(votes), int64(votesSum))
}

var oneHundredRat = big.NewRat(100, 1)

func FormatPercentage(percent *big.Rat) string {
	p := new(big.Rat)
	p.Mul(percent, oneHundredRat)
	return p.FloatString(3)
}
