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
	"reflect"
)

type AbstractPollSkeleton interface{}

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

func NewPollSkeletonCollection(title string) *PollSkeletonCollection {
	return &PollSkeletonCollection{
		Title:  title,
		Groups: make([]*PollGroup, 0),
	}
}

func (res *PollSkeletonCollection) getLastPollGroup() *PollGroup {
	if len(res.Groups) == 0 {
		panic("Internal error: Expected a group, but group list was empty!")
	}
	return res.Groups[len(res.Groups)-1]
}
