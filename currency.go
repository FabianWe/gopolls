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
	"regexp"
	"strconv"
	"strings"
)

// CurrencyValue represents a money value in a certain currency.
// The value is always represented as "cents", for example 1.23 € would be represented
// as ValueCents=123 and currency "€".
//
// There are also interfaces defined for formatting / parsing currency values.
type CurrencyValue struct {
	ValueCents int
	Currency   string
}

// NewCurrencyValue returns a new CurrencyValue.
func NewCurrencyValue(valueCents int, currency string) CurrencyValue {
	return CurrencyValue{
		ValueCents: valueCents,
		Currency:   currency,
	}
}

func (value CurrencyValue) String() string {
	return fmt.Sprintf("CurrencyValue{ValueCents: %d, Currency: %s}", value.ValueCents, value.Currency)
}

// Equals tests if two CurrencyValue objects are identical.
//
// Not that this method does not do "semantic" comparison on the currency, for example one could say that
// {42, "€"} is equal to {42, "EUR"}.
// This function however directly compares ValueCents and Currency.
func (value CurrencyValue) Equals(other CurrencyValue) bool {
	return value.ValueCents == other.ValueCents &&
		value.Currency == other.Currency
}

// Copy creates a copy of the value with exactly the same content.
func (value CurrencyValue) Copy() CurrencyValue {
	return CurrencyValue{
		ValueCents: value.ValueCents,
		Currency:   value.Currency,
	}
}

// DefaultFormatString returns a standard format and might be useful for formatters.
// It returns strings of the form 0.09, 0.21, 21.42 €.
// The separator (in the examples the dot) can be configured with sep.
func (value CurrencyValue) DefaultFormatString(sep string) string {
	if value.ValueCents < 0 {
		positiveValue := CurrencyValue{
			ValueCents: -value.ValueCents,
			Currency:   value.Currency,
		}
		return "-" + positiveValue.DefaultFormatString(sep)
	}
	currencyStr := ""
	if value.Currency != "" {
		currencyStr = " " + value.Currency
	}
	switch {
	case value.ValueCents < 10:
		return fmt.Sprintf("0%s0%d%s", sep, value.ValueCents, currencyStr)
	case value.ValueCents < 100:
		return fmt.Sprintf("0%s%d%s", sep, value.ValueCents, currencyStr)
	default:
		fullEuro := value.ValueCents / 100
		remainingCents := value.ValueCents % 100
		if remainingCents < 10 {
			return fmt.Sprintf("%d%s0%d%s", fullEuro, sep, remainingCents, currencyStr)
		}
		return fmt.Sprintf("%d%s%d%s", fullEuro, sep, remainingCents, currencyStr)
	}
}

// CurrencyFormatter formats a currency value to a string.
type CurrencyFormatter interface {
	Format(value CurrencyValue) string
}

// CurrencyParser parses a currency value from a string, error should be of type PollingSyntaxError or
// PollingSemanticError.
type CurrencyParser interface {
	Parse(s string) (CurrencyValue, error)
}

// CurrencyHandler combines formatter and parser in one interface.
//
// A general rule of thumb is: If a formatter returns a string representation for a currency value that same currency
// value should be parsed correctly back without errors.
type CurrencyHandler interface {
	CurrencyFormatter
	CurrencyParser
}

// SimpleEuroHandler is an implementation of CurrencyHandler (and thus CurrencyFormatter and CurrencyParser).
//
//
// It returns always strings of the form "1.23 €" or "1.23" (depending on whether Currency is set to an empty string
// or not).
// The parser allows strings of the form "42€", "21.42 €", "-42€", "21,42 €" (both , and . are allowed to be used as
// decimal separator, no thousands separator is supported).
type SimpleEuroHandler struct{}

var (
	// DefaultCurrencyHandler is the default CurrencyHandler, it is a SimpleEuroHandler, but it is not guaranteed
	// that this never changes.
	DefaultCurrencyHandler CurrencyHandler = SimpleEuroHandler{}
)

// Format implements the CurrencyFormatter interface.
func (h SimpleEuroHandler) Format(value CurrencyValue) string {
	return value.DefaultFormatString(".")
}

// simpleEuroRx is the regex used to parse values in with the SimpleEuroHandler.
var simpleEuroRx = regexp.MustCompile(`^\s*(-)?\s*(\d+)(?:[,.](\d{1,2}))?\s*(€)?\s*$`)

// Parse implements the CurrencyParser interface.
func (h SimpleEuroHandler) Parse(s string) (CurrencyValue, error) {
	res := CurrencyValue{}
	match := simpleEuroRx.FindStringSubmatch(s)
	if len(match) == 0 {
		return res, NewPollingSyntaxError(nil, "not a valid currency string: %s", s)
	}
	minus, euroStr, centsStr, currencySymbol := match[1], match[2], match[3], match[4]
	// try to parse fullEuroCents string first
	fullEuroCents, euroErr := strconv.Atoi(euroStr)
	if euroErr != nil {
		// in nearly all other cases we panic because of invalid syntax, in this case
		// not (sequence \d too long for int, seldom but could legally happen)
		return res, NewPollingSyntaxError(euroErr, "invalid currency integer")
	}
	fullEuroCents *= 100

	// now add cent if any given
	cents := 0
	if len(centsStr) > 0 {
		var centErr error
		cents, centErr = strconv.Atoi(centsStr)
		if centErr != nil {
			panic("Internal error in SimpleEuroHandler.Parse: cant parse cents as int, this should not happen, error: " + centErr.Error())
		}
	}

	// depending on the length of the string we have to multiply with 10
	switch len(centsStr) {
	case 0:
		// nothing to do, just fine
		break
	case 1:
		// multiply with 10
		fullEuroCents += cents * 10
	case 2:
		fullEuroCents += cents
	default:
		panic("Internal error in SimpleEuroHandler.Parse: cents must be string of length 1 or 2)")
	}

	// now look if value is negative
	if minus == "-" {
		fullEuroCents *= -1
	}

	res.ValueCents = fullEuroCents
	res.Currency = currencySymbol

	return res, nil
}

// RawCentCurrencyHandler implements CurrencyHandler.
// In th Parse method it accepts plain integers and reads them as plain integers, no currency
// symbol is allowed there.
// So the integer 10 would be translated to a currencly value "0.10" (10 cents).
// In its Format method it returns DefaultFormatString with . as separator.
type RawCentCurrencyHandler struct{}

func NewRawCentCurrencyParser() RawCentCurrencyHandler {
	return RawCentCurrencyHandler{}
}

func (h RawCentCurrencyHandler) Parse(s string) (CurrencyValue, error) {
	res := CurrencyValue{}
	s = strings.TrimSpace(s)
	intVal, intErr := strconv.Atoi(s)
	if intErr != nil {
		return res, NewPollingSyntaxError(intErr, "invalid currency integer")
	}
	res.ValueCents = intVal
	return res, nil
}

func (h RawCentCurrencyHandler) Format(value CurrencyValue) string {
	return value.DefaultFormatString(".")
}
