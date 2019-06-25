// Package util defines common models and utility functions.
package util

import (
	"regexp"
	"strings"
)

// Glob2RE translates a globbed string and translates it into a regular expression.
//
// the '*' character is translated into '.*'
//
// the '?' character is translated into '.'
//
// All other characters that have a meaning to regexp are escaped.
func Glob2RE(s string) (*regexp.Regexp, error) {
	if s[0] == '^' {
		return regexp.Compile(s)
	}
	rs := regexp.QuoteMeta(s)
	rs = strings.Replace(rs, `\*`, `.*`, -1)
	rs = strings.Replace(rs, `\?`, `.`, -1)
	return regexp.Compile("^" + rs + "$")
}

// Reader is implemented by all source formats that netwrangler
// understands
type Reader interface {
	Compile([]Phy) (*Layout, error)
	Read(string, []Phy) (*Layout, error)
}

// Writer is implemented by all target formats that netwrangler understands
type Writer interface {
	Write(string) error
	BindMacs()
}
