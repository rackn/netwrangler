package util

import (
	"fmt"
	"strings"
)

// Err is used to allow code to pile up errors for validation and
// reporting purposes.
type Err struct {
	Prefix string
	msgs   []string
}

// Errorf adds a new msg to an *Err
func (e *Err) Errorf(s string, args ...interface{}) {
	if e.msgs == nil {
		e.msgs = []string{}
	}
	e.msgs = append(e.msgs, fmt.Sprintf(s, args...))
}

// Error satisfies the error interface
func (e *Err) Error() string {
	res := []string{}
	res = append(res, fmt.Sprintf("%s:", e.Prefix))
	res = append(res, e.msgs...)
	res = append(res, "\n")
	return strings.Join(res, "\n")
}

// Empty returns whether any messages have been added to this Err
func (e *Err) Empty() bool {
	return e.msgs == nil || len(e.msgs) == 0
}

// Merge merges an error into this Err.  If other is an *Err, its
// messages will be appended to ours.
func (e *Err) Merge(other error) {
	if other == nil {
		return
	}
	if e.msgs == nil {
		e.msgs = []string{}
	}
	if o, ok := other.(*Err); ok {
		for _, msg := range o.msgs {
			e.Errorf("%s: %s", o.Prefix, msg)
		}
	} else {
		e.msgs = append(e.msgs, other.Error())
	}
}

// OrNil returns nil if the Err has no messages, the Err in question
// otherwise.
func (e *Err) OrNil() error {
	if e.Empty() {
		return nil
	}
	return e
}
