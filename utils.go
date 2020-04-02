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
	"math"
	"strconv"
)

// Weight is the type used to reference voter weights.
type Weight uint64

// NoWeight is a value used to signal that a value is not a valid Weight, for example as default argument.
const NoWeight Weight = math.MaxUint64

// ParseWeight parses a Weight from a string.
//
// An error is returned if weight is no valid int or is NoWeight.
func ParseWeight(s string) (Weight, error) {
	asInt, err := strconv.ParseUint(s, 10, 64)
	if err != nil {
		return 0, err
	}
	res := Weight(asInt)
	if res == NoWeight {
		return NoWeight, fmt.Errorf("integer value %d is too big", NoWeight)
	}
	return res, nil
}
