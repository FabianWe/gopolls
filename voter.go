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
)

// Voter implements everyone who is allowed to participate in polls.
//
// A voter has a name and weight. The weight specifies how much the vote of a certain voter counts (in "normal
//elections" this is 1).
type Voter struct {
	Name   string
	Weight Weight
}

// NewVoter creates a new Voter given its name and weight.
func NewVoter(name string, weight Weight) *Voter {
	return &Voter{
		Name:   name,
		Weight: weight,
	}
}

// Format returns a formatted string (one that can be parsed back with the voters parsing methods).
func (voter *Voter) Format(indent string) string {
	return fmt.Sprintf("%s* %s: %d", indent, voter.Name, voter.Weight)
}

// Equals tests if two voters are equal (have the same name and weight).
func (voter *Voter) Equals(other *Voter) bool {
	return voter.Name == other.Name && voter.Weight == other.Weight
}

// HasDuplicateVoters tests if there are duplicate names in a given voters list.
// It returns false if there are no duplicates, otherwise the first name that was found multiple times is returned
// together with true.
func HasDuplicateVoters(voters []*Voter) (string, bool) {
	nameSet := make(map[string]struct{}, len(voters))
	for _, voter := range voters {
		if _, has := nameSet[voter.Name]; has {
			return voter.Name, true
		}
		nameSet[voter.Name] = struct{}{}
	}
	return "", false
}

// VotersMap is a mapping from user name to a Voter.
type VotersMap map[string]*Voter

// VotersToMap returns a map from voter name to voter object.
// If it finds a a duplicate in the names of voters it returns nil and a DuplicateError.
func VotersToMap(voters []*Voter) (VotersMap, error) {
	res := make(VotersMap, len(voters))
	for _, voter := range voters {
		if _, has := res[voter.Name]; has {
			return nil, NewDuplicateError(fmt.Sprintf("duplicate entry for user %s", voter.Name))
		}
		res[voter.Name] = voter
	}
	return res, nil
}
