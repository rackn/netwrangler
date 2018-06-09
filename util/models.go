// Package util defines common models and utility functions.
package util

import (
	"fmt"
	"net"
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
func Glob2RE(s string) *regexp.Regexp {
	rs := regexp.QuoteMeta(s)
	rs = strings.Replace(rs, `\*`, `.*`, -1)
	rs = strings.Replace(rs, `\?`, `.`, -1)
	return regexp.MustCompile("^" + rs + "$")
}

// Reader is implemented by all source formats that netwrangler
// understands
type Reader interface {
	Read(string) (*Layout, error)
}

// Writer is implemented by all target formats that netwrangler understands
type Writer interface {
	Write(string) error
}

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

// IP is an alias of net.IPNet that we use to enable easy marshalling
// amd unmarshalling of IP addresses with and without CIDR
// specifications.
type IP net.IPNet

// UnmarshalText handles unmarshalling the string represenation of an
// IP address (v4 and v6, in CIDR form and as a raw address) into an
// IP.
func (i *IP) UnmarshalText(buf []byte) error {
	addr, cidr, err := net.ParseCIDR(string(buf))
	if err == nil {
		i.IP = addr
		i.Mask = cidr.Mask
		return nil
	}
	i.IP = net.ParseIP(string(buf))
	i.Mask = nil
	return nil
}

// MarshalText handles marshalling an IP into the appropriate text
// format.
func (i *IP) MarshalText() ([]byte, error) {
	return []byte(i.String()), nil
}

// IsCIDR returns whether this IP is in CIDR form.
func (i *IP) IsCIDR() bool {
	return len(i.Mask) > 0
}

// String() lets IP satisfy the Stringer interface.
func (i *IP) String() string {
	if len(i.Mask) == 0 {
		return i.IP.String()
	}
	return (*net.IPNet)(i).String()
}

// HardwareAddr is an alias of net.HardwareAddr that enables easy
// marshalling and unmarshalling of hardware addresses.
type HardwareAddr net.HardwareAddr

// MarshalText marshalls a HardwareAddr into a canonical string
// format.
func (h HardwareAddr) MarshalText() ([]byte, error) {
	return []byte(h.String()), nil
}

// UnmarshalText unmarshalls the text represenatation of a
// HardwareAddr.  Any format accepted by net.ParseMAC will be
// accepted.
func (h *HardwareAddr) UnmarshalText(buf []byte) error {
	mac, err := net.ParseMAC(string(buf))
	if err != nil {
		return err
	}
	*h = HardwareAddr(mac)
	return nil
}

// String lets HardwareAddr satisfy the Stringer interface.
func (h HardwareAddr) String() string {
	return net.HardwareAddr(h).String()
}
