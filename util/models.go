// Package util defines common models and utility functions.
package util

import (
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
	BindMacs()
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

// String() lets IP satisfy the Stringer interface.
func (i *IP) String() string {
	if len(i.Mask) == 0 {
		return i.IP.String()
	}
	return (*net.IPNet)(i).String()
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

// HardwareAddr is an alias of net.HardwareAddr that enables easy
// marshalling and unmarshalling of hardware addresses.
type HardwareAddr net.HardwareAddr

// String lets HardwareAddr satisfy the Stringer interface.
func (h HardwareAddr) String() string {
	return net.HardwareAddr(h).String()
}

// MarshalText marshalls a HardwareAddr into the canonical string
// format for a net.HardwareAddr
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
