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
