// Copyright 2020, 2021 Fabian Wenzelmann <fabianwen@posteo.eu>
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

package tests

import (
	"github.com/FabianWe/gopolls"
	"testing"
)

func TestSimpleEuroHandlerFormat(t *testing.T) {
	tests := []struct {
		in       int
		expected string
	}{
		{1, "0.01 €"},
		{99, "0.99 €"},
		{100, "1.00 €"},
		{4209, "42.09 €"},
		{-42, "-0.42 €"},
		{-1337, "-13.37 €"},
	}

	handler := gopolls.SimpleEuroHandler{}
	for _, tc := range tests {
		currency := gopolls.CurrencyValue{
			ValueCents: tc.in,
			Currency:   "€",
		}
		got := handler.Format(currency)
		if got != tc.expected {
			t.Errorf("For input %d expected format string to be \"%s\", got \"%s\" instead",
				tc.in, tc.expected, got)
		}

	}
}

func TestSimpleEuroHandlerParse(t *testing.T) {
	tests := []struct {
		in       string
		expected gopolls.CurrencyValue
	}{
		{"0", gopolls.NewCurrencyValue(0, "")},
		{"1", gopolls.NewCurrencyValue(100, "")},
		{"42 €", gopolls.NewCurrencyValue(4200, "€")},
		{"100,00", gopolls.NewCurrencyValue(10000, "")},
		{"42,21 €", gopolls.NewCurrencyValue(4221, "€")},
		{"42,09€", gopolls.NewCurrencyValue(4209, "€")},
		{"-42,09    €", gopolls.NewCurrencyValue(-4209, "€")},
		{"-0,21", gopolls.NewCurrencyValue(-21, "")},
	}

	handler := gopolls.SimpleEuroHandler{}
	for _, tc := range tests {
		parsed, parsedErr := handler.Parse(tc.in)
		if parsedErr != nil {
			t.Errorf("Unexpected error while parsing \"%s\": %v", tc.in, parsedErr)
			continue
		}
		if !tc.expected.Equals(parsed) {
			t.Errorf("For input \"%s\" epxected output %s, but got %s", tc.in, tc.expected, parsed)
		}
	}
}
