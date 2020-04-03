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
)

type CurrencyValue struct {
	ValueCents int
	Currency   string
}

func NewCurrencyValue(valueCents int, currency string) CurrencyValue {
	return CurrencyValue{
		ValueCents: valueCents,
		Currency:   currency,
	}
}

func (value CurrencyValue) String() string {
	return fmt.Sprintf("CurrencyValue{ValueCents: %d, Currency: %s}", value.ValueCents, value.Currency)
}

func (value CurrencyValue) Equals(other CurrencyValue) bool {
	return value.ValueCents == other.ValueCents &&
		value.Currency == other.Currency
}

type CurrencyFormatter interface {
	Format(value CurrencyValue) string
}

type CurrencyParser interface {
	Parse(s string) (CurrencyValue, error)
}

type CurrencyHandler interface {
	CurrencyFormatter
	CurrencyParser
}

type SimpleEuroHandler struct{}

func (h SimpleEuroHandler) Format(value CurrencyValue) string {
	if value.ValueCents < 0 {
		positiveValue := CurrencyValue{
			ValueCents: -value.ValueCents,
			Currency:   value.Currency,
		}
		return "-" + h.Format(positiveValue)
	}
	currencyStr := ""
	// this simple formatter does only allow €
	if value.Currency != "" {
		currencyStr = " €"
	}
	switch {
	case value.ValueCents < 10:
		return fmt.Sprintf("0,0%d%s", value.ValueCents, currencyStr)
	case value.ValueCents < 100:
		return fmt.Sprintf("0,%d%s", value.ValueCents, currencyStr)
	default:
		fullEuro := value.ValueCents / 100
		remainingCents := value.ValueCents % 100
		if remainingCents < 10 {
			return fmt.Sprintf("%d,0%d%s", fullEuro, remainingCents, currencyStr)
		}
		return fmt.Sprintf("%d,%d%s", fullEuro, remainingCents, currencyStr)
	}
}

var simpleEuroRx = regexp.MustCompile(`^\s*(-)?\s*(\d+)(?:[,.](\d{1,2}))?\s*(€)?\s*$`)

func (h SimpleEuroHandler) Parse(s string) (CurrencyValue, error) {
	res := CurrencyValue{}
	match := simpleEuroRx.FindStringSubmatch(s)
	if len(match) == 0 {
		return res, fmt.Errorf("not a valid currency string: %s", s)
	}
	minus, euroStr, centsStr, currencySymbol := match[1], match[2], match[3], match[4]
	// try to parse fullEuroCents string first
	fullEuroCents, euroErr := strconv.Atoi(euroStr)
	if euroErr != nil {
		// in nearly all other cases we panic because of invalid syntax, in this case
		// not (sequence \d too long for int, seldom but could legally happen)
		return res, euroErr
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
