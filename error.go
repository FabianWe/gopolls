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

// PollError is an error returned from all method indicating an error from inside gopolls.
// Usually errors like syntax error are just wrapped in a PollError while errors from reading from a stream etc.
// are returned directly.
// This way you can check if the error was caused by gopolls or if something went wrong while reading / writing
// to a certain source.
type PollError struct {
	// The actual error
	Err error
}

// NewPollError returns a new PollError.
func NewPollError(actual error) *PollError {
	return &PollError{Err: actual}
}

func (err PollError) Error() string {
	return err.Err.Error()
}

func (err PollError) Unwrap() error {
	return err.Err
}
