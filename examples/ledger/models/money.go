package models

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// Money is an amount in minor units, such as cents. Floats cannot hold 0.10, so
// money is an integer, in Go and in the database, and only reads as a decimal in
// JSON. It is a named int64, so it maps to bigint, and ant's arithmetic works on it.
type Money int64

// MarshalJSON writes "12.34", a string because JSON numbers are floats to most
// readers.
func (m Money) MarshalJSON() ([]byte, error) { return json.Marshal(m.String()) }

// UnmarshalJSON reads "12.34".
func (m *Money) UnmarshalJSON(data []byte) error {
	var text string
	if err := json.Unmarshal(data, &text); err != nil {
		return fmt.Errorf("an amount is a string such as \"12.34\": %w", err)
	}

	parsed, err := ParseMoney(text)
	*m = parsed

	return err
}

func (m Money) String() string {
	sign, value := "", int64(m)
	if value < 0 {
		sign, value = "-", -value
	}

	return fmt.Sprintf("%s%d.%02d", sign, value/100, value%100)
}

// ParseMoney reads a decimal with at most two places.
func ParseMoney(text string) (Money, error) {
	whole, fraction, _ := strings.Cut(strings.TrimSpace(text), ".")
	negative := strings.HasPrefix(whole, "-")
	whole = strings.TrimPrefix(whole, "-")

	if len(fraction) > 2 {
		return 0, fmt.Errorf("%q has more than two decimal places", text)
	}

	fraction = (fraction + "00")[:2]

	units, err := strconv.ParseInt(whole, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("%q is not an amount", text)
	}

	cents, err := strconv.ParseInt(fraction, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("%q is not an amount", text)
	}

	out := units*100 + cents

	if negative {
		out = -out
	}

	return Money(out), nil
}
