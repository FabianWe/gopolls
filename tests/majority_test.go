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
	"math/big"
	"testing"
)

func TestComputeMajority(t *testing.T) {
	tests := []struct {
		majority *big.Rat
		votesSum gopolls.Weight
		expected gopolls.Weight
	}{
		{gopolls.FiftyPercentMajority, 10, 5},
		{gopolls.TwoThirdsMajority, 10, 6},
		{big.NewRat(50, 100), 10, 5},
		{gopolls.FiftyPercentMajority, 0, 0},
		{big.NewRat(0, 1), 42, 0},
		{gopolls.FiftyPercentMajority, 42, 21},
		{gopolls.TwoThirdsMajority, 42, 28},
		{big.NewRat(1, 3), 42, 14},
		{big.NewRat(2, 2), gopolls.NoWeight, gopolls.NoWeight},
	}

	for _, tc := range tests {
		res := gopolls.ComputeMajority(tc.majority, tc.votesSum)
		if res != tc.expected {
			t.Errorf("expected that the required majority for %s and %d to be %d, but got %d",
				tc.majority, tc.votesSum, tc.expected, res)
		}
	}
}

func TestComputePercentage(t *testing.T) {
	tests := []struct {
		votes, total gopolls.Weight
		expected     *big.Rat
	}{
		{1, 1, big.NewRat(1, 1)},
		{1, 2, big.NewRat(1, 2)},
		{42, 0, big.NewRat(0, 1)},
		{10, 20, big.NewRat(1, 2)},
	}

	for _, tc := range tests {
		percentage := gopolls.ComputePercentage(tc.votes, tc.total)
		if percentage.Cmp(tc.expected) != 0 {
			t.Errorf("Expected percentage for input %d and %d to be %s, but got %s",
				tc.votes, tc.total, tc.expected, percentage)
		}
	}
}

func TestFormatPercentage(t *testing.T) {
	tests := []struct {
		in       *big.Rat
		expected string
	}{
		{big.NewRat(0, 1), "0.000"},
		{big.NewRat(1, 2), "50.000"},
		{big.NewRat(3, 10), "30.000"},
		{big.NewRat(3, 4), "75.000"},
		{big.NewRat(1, 3), "33.333"},
	}
	for _, tc := range tests {
		got := gopolls.FormatPercentage(tc.in)
		if got != tc.expected {
			t.Errorf("Expected format of %s to be %s, but got %s instead",
				tc.in, tc.expected, got)
		}
	}
}
