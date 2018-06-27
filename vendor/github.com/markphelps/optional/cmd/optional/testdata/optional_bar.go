// Code generated by go generate
// This file was generated by robots at 2018-03-27 01:14:21.59761375 +0000 UTC

package bar

import (
	"encoding/json"
	"errors"
)

// optionalBar is an optional bar
type optionalBar struct {
	value *bar
}

// NewoptionalBar creates a optional.optionalBar from a bar
func NewoptionalBar(v bar) optionalBar {
	return optionalBar{&v}
}

// Set sets the bar value
func (o optionalBar) Set(v bar) {
	o.value = &v
}

// Get returns the bar value or an error if not present
func (o optionalBar) Get() (bar, error) {
	if !o.Present() {
		return *o.value, errors.New("value not present")
	}
	return *o.value, nil
}

// Present returns whether or not the value is present
func (o optionalBar) Present() bool {
	return o.value != nil
}

// OrElse returns the bar value or a default value if the value is not present
func (o optionalBar) OrElse(v bar) bar {
	if o.Present() {
		return *o.value
	}
	return v
}

// If calls the function f with the value if the value is present
func (o optionalBar) If(fn func(bar)) {
	if o.Present() {
		fn(*o.value)
	}
}

func (o optionalBar) MarshalJSON() ([]byte, error) {
	if o.Present() {
		return json.Marshal(o.value)
	}
	var zero bar
	return json.Marshal(zero)
}

func (o *optionalBar) UnmarshalJSON(data []byte) error {
	var value bar

	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}

	o.value = &value
	return nil
}
