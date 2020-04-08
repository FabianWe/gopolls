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

// internalErrorSentinelType is used only for the constant "ErrPoll", this way we have one sentinel value
// to expose.
// The type PollError tests for this constant in its Is(error) method.
type internalErrorSentinelType struct{}

// The type must implement the error interface.
func (err internalErrorSentinelType) Error() string {
	return "gopolls error"
}

// ErrPoll is a constant that can be used with a type check.
// All internal errors (such as syntax error) can be used in a statement like erorrs.Is(err, ErrPoll)
// and return true.
// This can be useful when you want to distinguish between an error from gopolls and an "outside" error.
// If you want to dig deeper, for example find out if an error is a syntax error, you should use
// errors.As(err, *ERROR_TYPE).
var ErrPoll = internalErrorSentinelType{}

// PollError is an error used for errors that should be considered a polling error, such as syntax
// error, evaluation errors for your own poll types etc.
// The type itself does not implement the error interface, but only the method Is(err error) from the error
// package.
// This way you can just embed this type in your own error type and Is(err, ErrPoll) will return true.
type PollError struct{}

// Is returns true if err == ErrPoll.
func (internal PollError) Is(err error) bool {
	return err == ErrPoll
}
