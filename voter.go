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

// Voter implements everyone who is allowed to participate in polls.
//
// A voter has a name and weight. The weight specifies how much the count of a certain voter counts
// (in normal "elections" this is 1).
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
