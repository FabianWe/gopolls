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

type AbstractPollSkeleton interface{}

func DumpAbstractPollSkeleton(skel AbstractPollSkeleton, w io.Writer, currencyFormatter CurrencyFormatter) (int, error) {
	switch typedSkel := skel.(type) {
	case *MoneyPollSkeleton:
		return typedSkel.Dump(w, currencyFormatter)
	case *PollSkeleton:
		return typedSkel.Dump(w)
	default:
		return 0, fmt.Errorf("Skeleton must be either *MoneyPollSkeleton or *PollSkeleton, got type %s",
			reflect.TypeOf(skel))
	}
}

type MoneyPollSkeleton struct {
	Name  string
	Value CurrencyValue
}

func NewMoneyPollSkeleton(name string, value CurrencyValue) *MoneyPollSkeleton {
	return &MoneyPollSkeleton{
		Name:  name,
		Value: value,
	}
}

func (skel *MoneyPollSkeleton) Dump(w io.Writer, currencyFormatter CurrencyFormatter) (int, error) {
	currencyString := currencyFormatter.Format(skel.Value)
	return fmt.Fprintf(w, "### %s\n- %s\n\n", skel.Name, currencyString)
}

type PollSkeleton struct {
	Name    string
	Options []string
}

func NewPollSkeleton(name string) *PollSkeleton {
	return &PollSkeleton{
		Name:    name,
		Options: make([]string, 0, 2),
	}
}

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

type PollGroup struct {
	Title     string
	Skeletons []AbstractPollSkeleton
}

func NewPollGroup(title string) *PollGroup {
	return &PollGroup{
		Title:     title,
		Skeletons: make([]AbstractPollSkeleton, 0),
	}
}

func (group *PollGroup) NumSkeletons() int {
	return len(group.Skeletons)
}

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

func (group *PollGroup) getLastPoll() *PollSkeleton {
	if len(group.Skeletons) == 0 {
		panic("Internal error: Expected a money poll on parse list, list was empty!")
	}
	last := group.Skeletons[len(group.Skeletons)-1]
	asPoll, ok := last.(*PollSkeleton)
	if !ok {
		panic(fmt.Sprintf("Internal error: Expected a poll on parse list, got type %s instead!", reflect.TypeOf(last)))
	}
	return asPoll
}

type PollSkeletonCollection struct {
	Title  string
	Groups []*PollGroup
}

func (coll *PollSkeletonCollection) NumGroups() int {
	return len(coll.Groups)
}

func (coll *PollSkeletonCollection) NumSkeletons() int {
	res := 0
	for _, group := range coll.Groups {
		res += group.NumSkeletons()
	}
	return res
}

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

func NewPollSkeletonCollection(title string) *PollSkeletonCollection {
	return &PollSkeletonCollection{
		Title:  title,
		Groups: make([]*PollGroup, 0),
	}
}

func (coll *PollSkeletonCollection) getLastPollGroup() *PollGroup {
	if len(coll.Groups) == 0 {
		panic("Internal error: Expected a group, but group list was empty!")
	}
	return coll.Groups[len(coll.Groups)-1]
}
