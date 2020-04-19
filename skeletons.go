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
	"io"
	"reflect"
)

const (
	MoneyPollSkeletonType   = "money-skeleton"
	GeneralPollSkeletonType = "basic-skeleton"
)

// AbstractPollSkeleton describes a poll skeleton or "framework".
// Skeletons can be mapped to a poll instance with a SkeletonConverter.
// The reason for having skeletons and polls is that a description might be interpreted differently and there are
// various methods for evaluating a poll
// For example the Schulze method or any other procedure.
//
// Thus a skeleton is the description (which options are there etc.) while the poll itself contains the logic of
// evaluating it.
//
// We have two different skeletons at the moment (but three poll implementations:
//
// MoneyPollSkeleton, has a name and a money value, is usually converted to a MedianPoll.
//
// PollSkeleton, has a name and a list of options.
// It is usually converted to either a SchulzePoll (> 2 options) or a BasicPoll (two options).
// The converter implemented at the moment, DefaultSkeletonConverter, will do the translations mentioned above,
// but other ones are possible too.
type AbstractPollSkeleton interface {
	SkeletonType() string
	GetName() string
}

// PollSkeletonMap is a map from a poll name to the poll skeleton with that name.
type PollSkeletonMap map[string]AbstractPollSkeleton

// DumpAbstractPollSkeleton writes a skeleton description to a writer.
// It works only with the two "default" implementations.
//
// It returns the number of bytes written and any error that occurred writing to w.
//
// It needs a currencyFormatter to write MoneyPollSkeleton instances.
func DumpAbstractPollSkeleton(skel AbstractPollSkeleton, w io.Writer, currencyFormatter CurrencyFormatter) (int, error) {
	switch typedSkel := skel.(type) {
	case *MoneyPollSkeleton:
		return typedSkel.Dump(w, currencyFormatter)
	case *PollSkeleton:
		return typedSkel.Dump(w)
	default:
		return 0, NewPollTypeError("skeleton must be either *MoneyPollSkeleton or *PollSkeleton, got type %s",
			reflect.TypeOf(skel))
	}
}

// MoneyPollSkeleton is an AbstractPollSkeleton for a poll about some currency value (money).
type MoneyPollSkeleton struct {
	Name  string
	Value CurrencyValue
}

// NewMoneyPollSkeleton returns a new MoneyPollSkeleton.
func NewMoneyPollSkeleton(name string, value CurrencyValue) *MoneyPollSkeleton {
	return &MoneyPollSkeleton{
		Name:  name,
		Value: value,
	}
}

// Dump writes the skeleton to some writer w, it needs a currencyFormatter to write currency values.
//
// It returns the number of bytes written as well as any error writing to w.
func (skel *MoneyPollSkeleton) Dump(w io.Writer, currencyFormatter CurrencyFormatter) (int, error) {
	currencyString := currencyFormatter.Format(skel.Value)
	return fmt.Fprintf(w, "### %s\n- %s\n\n", skel.Name, currencyString)
}

// SkeletonType returns the constant MoneyPollSkeletonType.
func (skel *MoneyPollSkeleton) SkeletonType() string {
	return MoneyPollSkeletonType
}

// GetName returns the name of the poll description.
func (skel *MoneyPollSkeleton) GetName() string {
	return skel.Name
}

// PollSkeleton is an AbstractPollSkeleton for a poll with a list of options (strings).
type PollSkeleton struct {
	Name    string
	Options []string
}

// NewPollSkeleton returns a new PollSkeleton given the name and an empty list of options.
func NewPollSkeleton(name string) *PollSkeleton {
	return &PollSkeleton{
		Name:    name,
		Options: make([]string, 0, 2),
	}
}

// Dump writes the skeleton to some writer w.
//
// It returns the number of bytes written as well as any error writing to w.
func (skel *PollSkeleton) Dump(w io.Writer) (int, error) {
	res := 0
	// re-used to store what currently has been written / error occurred
	written := 0
	var writeErr error

	written, writeErr = fmt.Fprintf(w, "### %s\n", skel.Name)
	res += written
	if writeErr != nil {
		return res, writeErr
	}

	for _, option := range skel.Options {
		written, writeErr = fmt.Fprintf(w, "* %s\n", option)
		res += written
		if writeErr != nil {
			return res, writeErr
		}
	}

	written, writeErr = fmt.Fprintln(w)
	res += written

	return res, writeErr
}

// SkeletonType returns the constant GeneralPollSkeletonType.
func (skel *PollSkeleton) SkeletonType() string {
	return GeneralPollSkeletonType
}

// GetName returns the name of the poll description.
func (skel *PollSkeleton) GetName() string {
	return skel.Name
}

// PollGroup is a group (collection) of votes.
//
// Polls are put into groups and a list of groups describes a poll collection.
type PollGroup struct {
	Title     string
	Skeletons []AbstractPollSkeleton
}

// NewPollGroup returns a new PollGroup with an empty list of skeletons.
func NewPollGroup(title string) *PollGroup {
	return &PollGroup{
		Title:     title,
		Skeletons: make([]AbstractPollSkeleton, 0, 8),
	}
}

// NumSkeletons returns the number of skeletons in this group.
func (group *PollGroup) NumSkeletons() int {
	return len(group.Skeletons)
}

// Dump writes this group to a writer, it needs a currencyFormatter to write money polls.
//
// It returns the number of bytes written as well as any error writing to w.
func (group *PollGroup) Dump(w io.Writer, currencyFormatter CurrencyFormatter) (int, error) {
	res := 0
	// re-used to store what currently has been written / error occurred
	written := 0
	var writeErr error
	written, writeErr = fmt.Fprintf(w, "## %s\n\n", group.Title)
	res += written
	if writeErr != nil {
		return res, writeErr
	}
	for _, pollSkel := range group.Skeletons {
		written, writeErr = DumpAbstractPollSkeleton(pollSkel, w, currencyFormatter)
		res += written
		if writeErr != nil {
			return res, writeErr
		}
	}

	return res, writeErr
}

// getLastPoll is used internally to retrieve the last poll in a group.
// If the polls list is empty it panics.
// The last poll must be of type *PollSkeleton, otherwise this function panics too.
// The parser (if it has no bugs) should take care to call this function only if these conditions are met.
func (group *PollGroup) getLastPoll() *PollSkeleton {
	if len(group.Skeletons) == 0 {
		panic("Internal error: Expected a money poll on parse list, list was empty!")
	}
	last := group.Skeletons[len(group.Skeletons)-1]
	asPoll, ok := last.(*PollSkeleton)
	if !ok {
		panic(fmt.Sprintf("internal error: Expected a poll on parse list, got type %s instead!", reflect.TypeOf(last)))
	}
	return asPoll
}

// PollSkeletonCollection describes a collection of polls that are divided into groups.
type PollSkeletonCollection struct {
	Title  string
	Groups []*PollGroup
}

// NewPollSkeletonCollection returns a new PollSkeletonCollection with an empty list of groups.
func NewPollSkeletonCollection(title string) *PollSkeletonCollection {
	return &PollSkeletonCollection{
		Title:  title,
		Groups: make([]*PollGroup, 0, 8),
	}
}

// PollSkeletonCollection
func (coll *PollSkeletonCollection) NumGroups() int {
	return len(coll.Groups)
}

// NumSkeletons returns the number of skeletons in all groups.
func (coll *PollSkeletonCollection) NumSkeletons() int {
	res := 0
	for _, group := range coll.Groups {
		res += group.NumSkeletons()
	}
	return res
}

// CollectSkeletons returns a list of all skeletons that appear in any of the groups.
func (coll *PollSkeletonCollection) CollectSkeletons() []AbstractPollSkeleton {
	res := make([]AbstractPollSkeleton, 0, len(coll.Groups))
	for _, group := range coll.Groups {
		res = append(res, group.Skeletons...)
	}
	return res
}

// HasDuplicateSkeleton tests if the names in the collection are unique (which they should).
// It returns an empty string and false if no duplicates where found, otherwise it returns the name
// of the skeleton and true.
func (coll *PollSkeletonCollection) HasDuplicateSkeleton() (string, bool) {
	nameSet := make(map[string]struct{}, len(coll.Groups))
	for _, group := range coll.Groups {
		for _, skel := range group.Skeletons {
			name := skel.GetName()
			if _, has := nameSet[name]; has {
				return name, true
			}
			nameSet[name] = struct{}{}
		}
	}
	return "", false
}

// SkeletonsToMap returns a map from skeleton name to skeleton.
// If it finds any duplicate names an error of type DuplicateError is returned together with nil.
//
// Otherwise it returns the map and nil.
func (coll *PollSkeletonCollection) SkeletonsToMap() (PollSkeletonMap, error) {
	res := make(PollSkeletonMap, len(coll.Groups))
	for _, group := range coll.Groups {
		for _, skel := range group.Skeletons {
			name := skel.GetName()
			if _, has := res[name]; has {
				return nil, NewDuplicateError(fmt.Sprintf("duplicate entry for poll %s", name))
			}
			res[name] = skel
		}
	}
	return res, nil
}

// Dump writes the collection to some writer w, it needs a currencyFormatter to write currency values.
//
// It returns the number of bytes written as well as any error writing to w.
func (coll *PollSkeletonCollection) Dump(w io.Writer, currencyFormatter CurrencyFormatter) (int, error) {
	res := 0
	// re-used to store what currently has been written / error occurred
	written := 0
	var writeErr error
	written, writeErr = fmt.Fprintf(w, "# %s\n\n", coll.Title)
	res += written
	if writeErr != nil {
		return res, writeErr
	}

	for _, group := range coll.Groups {
		written, writeErr = group.Dump(w, currencyFormatter)
		res += written
		if writeErr != nil {
			return res, writeErr
		}
	}

	return res, writeErr
}

// getLastPollGroup returns the last poll group. It is internally used in the parser.
// It panics of there are no groups yet. The parser (if implemented without bugs) should call this method
// only if there is at least one group.
func (coll *PollSkeletonCollection) getLastPollGroup() *PollGroup {
	if len(coll.Groups) == 0 {
		panic("Internal error: Expected a group, but group list was empty!")
	}
	return coll.Groups[len(coll.Groups)-1]
}
