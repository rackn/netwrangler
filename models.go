package netmangler

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"math"
	"net"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"

	yaml "github.com/ghodss/yaml"
)

func g2re(s string) *regexp.Regexp {
	rs := regexp.QuoteMeta(s)
	rs = strings.Replace(rs, `\*`, `.*`, -1)
	rs = strings.Replace(rs, `\?`, `.`, -1)
	return regexp.MustCompile("^" + rs + "$")
}

type err struct {
	prefix string
	msgs   []string
}

func (e *err) Errorf(s string, args ...interface{}) {
	if e.msgs == nil {
		e.msgs = []string{}
	}
	e.msgs = append(e.msgs, fmt.Sprintf(s, args...))
}

func (e *err) Error() string {
	res := []string{}
	res = append(res, fmt.Sprintf("%s:", e.prefix))
	res = append(res, e.msgs...)
	res = append(res, "\n")
	return strings.Join(res, "\n")
}

func (e *err) Empty() bool {
	return e.msgs == nil || len(e.msgs) == 0
}

func (e *err) Merge(other error) {
	if other == nil {
		return
	}
	if e.msgs == nil {
		e.msgs = []string{}
	}
	if o, ok := other.(*err); ok {
		for _, msg := range o.msgs {
			e.Errorf("%s: %s", o.prefix, msg)
		}
	} else {
		e.msgs = append(e.msgs, other.Error())
	}
}

func (e *err) OrNil() error {
	if e.Empty() {
		return nil
	}
	return e
}

type IP net.IPNet

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

func (i *IP) MarshalText() ([]byte, error) {
	if i.Mask == nil {
		return []byte(i.IP.String()), nil
	}
	return []byte((*net.IPNet)(i).String()), nil
}

func (i *IP) IsCIDR() bool {
	return i.Mask != nil
}

type HardwareAddr net.HardwareAddr

func (h HardwareAddr) MarshalText() ([]byte, error) {
	return []byte(net.HardwareAddr(h).String()), nil
}

func (h HardwareAddr) UnmarshalText(buf []byte) error {
	mac, err := net.ParseMAC(string(buf))
	if err != nil {
		return err
	}
	copy(h, mac)
	return nil
}

func validateBool(e *err, k string, v interface{}) (res, valid bool) {
	switch val := v.(type) {
	case bool:
		return val, true
	case string:
		switch val {
		case "0", "f", "false", "off":
			return false, true
		case "1", "t", "true", "on":
			return true, true
		}
	}
	e.Errorf("%s: Cannot cast %v to a boolean", k, v)
	return false, false
}

func validateInt(e *err, k string, v interface{}, min, max, def int) (res int, valid bool) {
	val := def
	switch v.(type) {
	case int64:
		val = int(v.(int64))
	case uint64:
		val = int(v.(uint64))
	case float64:
		val = int(v.(float64))
	case string:
		vv, err := strconv.ParseInt(v.(string), 0, 64)
		if err != nil {
			e.Errorf("%s: Cannot cast %v to an int: %v", k, v, err)
			return -1, false
		}
		val = int(vv)
	default:
		e.Errorf("%s: Cannot cast %T to an int: %v", k, v, v)
		return -1, false
	}
	if val == def {
		return val, false
	}
	if min > val || val > max {
		e.Errorf("%s: %d out of range %d:%d", k, val, min, max)
	}
	return val, true
}

func validateStrIn(e *err, k string, v interface{}, vals ...string) (res string, valid bool) {
	res, valid = v.(string)
	if !valid {
		e.Errorf("%s: %v is not a string", k, v)
		return
	}
	if len(vals) > 0 {
		for _, s := range vals {
			if res == s {
				return res, res == ""
			}
		}
		e.Errorf("%s: %s: Not in valid set: %v", k, res, valid)
		return res, false
	}
	return res, true
}

func validateIPList(e *err, k string, v interface{}, cidr bool) (res []*IP, valid bool) {
	valid = true
	buf, err := json.Marshal(v)
	if err != nil {
		valid = false
		e.Errorf("%s: Failed to marshal %v: %v", k, v, err)
		return
	}
	if err := json.Unmarshal(buf, &res); err != nil {
		valid = false
		e.Errorf("%s: Cannot cast %v to a list of IPs: %v", k, v, err)
		return
	}
	for _, addr := range res {
		if addr.IsCIDR() == cidr {
			continue
		}
		valid = false
		e.Errorf("%s: %v is not in the expected format", k, addr)
	}
	return
}

type Route struct {
	From   *IP    `json:"from"`
	To     *IP    `json:"to"`
	Via    *IP    `json:"via"`
	OnLink bool   `json:"on-link"`
	Metric uint64 `json:"metric"`
	Type   string `json:"type"`
	Scope  string `json:"scope"`
	Table  uint64 `json:"table"`
}

func (r *Route) Validate() error {
	return fmt.Errorf("Route Not Implemented")
}

type RoutePolicy struct {
	From     *IP    `json:"from"`
	To       *IP    `json:"to"`
	Table    uint64 `json:"table"`
	Priority uint64 `json:"priority"`
	FWMark   uint64 `json:"fwmark"`
	TOS      int64  `json:"type-of-service"`
}

func (r *RoutePolicy) Validate() error {
	return fmt.Errorf("RoutePolicy Not Implemented")
}

type Network struct {
	Dhcp4          bool   `json:"dhcp4,omitempty"`
	Dhcp6          bool   `json:"dhcp6,omitempty"`
	DhcpIdentifier string `json:"dhcp-identifier,omitempty"`
	SkipRa         bool   `json:"accept-ra"`
	Addresses      []*IP  `json:"addresses,omitempty"`
	Gateway4       *IP    `json:"gateway4,omitempty"`
	Gateway6       *IP    `json:"gateway6,omitempty"`
	Nameservers    struct {
		Search    []string `json:"search,omitempty"`
		Addresses []*IP    `json:"addresses,omitempty"`
	} `json:"nameservers,omitempty"`
	MacAddress    HardwareAddr  `json:"macaddress,omitempty"`
	Optional      bool          `json:"optional,omitempty"`
	Routes        []Route       `json:"routes,omitempty"`
	RoutingPolicy []RoutePolicy `json:"routing-policy,omitempty"`
}

func (n *Network) configure() bool {
	return n != nil
}

func (n *Network) SetupStaticOnly() bool {
	return n.configure() && !(n.Dhcp4 || n.Dhcp6) && len(n.Addresses) > 0
}

func (n *Network) Configure() bool {
	return n.configure() && (n.Dhcp4 || n.Dhcp6 || len(n.Addresses) != 0)
}

func (n *Network) SetupDHCPOnly() bool {
	return n.configure() && (n.Dhcp4 || n.Dhcp6) && len(n.Addresses) == 0
}

func (n *Network) Validate() error {
	e := &err{prefix: "network"}
	validateStrIn(e, "dhcp-identifier", n.DhcpIdentifier, "mac", "")
	n.SkipRa = !n.SkipRa
	if n.Addresses == nil {
		n.Addresses = []*IP{}
	}
	validateIPList(e, "addresses", n.Addresses, true)
	if n.Gateway4 != nil && n.Gateway4.IP.To4() == nil {
		e.Errorf("Gateway4 %s is not an IPv4 address", n.Gateway4)
	}
	if n.Gateway6 != nil && n.Gateway6.IP.To4() != nil {
		e.Errorf("Gateway6 %s is not an IPv6 address", n.Gateway6)
	}
	if n.Nameservers.Addresses != nil {
		validateIPList(e, "nameservers", n.Nameservers.Addresses, false)
	}
	if n.Routes != nil {
		for _, route := range n.Routes {
			e.Merge(route.Validate())
		}
	}
	if n.RoutingPolicy != nil {
		for _, rp := range n.RoutingPolicy {
			e.Merge(rp.Validate())
		}
	}
	return e.OrNil()
}

type Interface struct {
	*Network
	Type       string                 `json:"type"`
	MatchID    string                 `json:"match-id"`
	HwAddr     HardwareAddr           `json:"macaddress"`
	Interfaces []string               `json:"interfaces,omitempty"`
	Parameters map[string]interface{} `json:"parameters,omitempty"`
}

func (i *Interface) Validate() error {
	if i.Network != nil {
		return i.Network.Validate()
	}
	return nil
}

type Physical struct {
	Interface
	Match struct {
		Name       string       `json:"name,omitempty"`
		MacAddress HardwareAddr `json:"macaddress,omitempty"`
		Driver     string       `json:"driver,omitempty"`
	}
	WakeOnLan bool `json:"wakeonlan,omitempty"`
}

func (pi *Physical) MatchEmpty() bool {
	return pi.Match.Name == "" &&
		pi.Match.Driver == "" &&
		(pi.Match.MacAddress == nil || len(pi.Match.MacAddress) == 0)
}

func (pi *Physical) Validate() error {
	pi.Type = "physical"
	e := &err{prefix: pi.Type + ":" + pi.MatchID}
	pi.Parameters["wakeonlan"] = pi.WakeOnLan
	e.Merge(pi.Interface.Validate())
	pi.Interfaces = []string{}
	for _, intf := range phys {
		nre := g2re(pi.Match.Name)
		if !(nre.MatchString(intf.Name) || nre.MatchString(intf.StableName)) {
			continue
		}
		if !(pi.Match.MacAddress == nil || len(pi.Match.MacAddress) == 0) {
			if !bytes.Equal(pi.Match.MacAddress, intf.HwAddr) {
				continue
			}
		}
		if pi.Match.Driver != "" && pi.Match.Driver != intf.Driver {
			continue
		}
		// We have a potential match!
		if other, ok := claimedPhys[intf.Name]; ok {
			// but some other network wants it
			e.Errorf("Physical link %s wants physical interface %s, but so does %s", pi.MatchID, intf.Name, other.MatchID)
			continue
		}
		pi.Interfaces = append(pi.Interfaces, intf.Name)
		claimedPhys[intf.Name] = pi.Interface
	}
	log.Printf("%s: subs %v", pi.MatchID, pi.Interfaces)
	switch len(pi.Interfaces) {
	case 0:
		e.Errorf("Ethernet network %s does not match any physical interfaces!", pi.MatchID)
	case 1:
	default:
		if pi.Configure() && !pi.SetupDHCPOnly() {
			e.Errorf("Network matches multiple devices %v, cannot configure them all with static networking!", pi.Interfaces)
		}
	}
	return e.OrNil()
}

type Bridge struct{ Interface }

func (br *Bridge) Validate() error {
	br.Type = "bridge"
	e := &err{prefix: br.Type + ":" + br.MatchID}
	e.Merge(br.Interface.Validate())

	params := map[string]interface{}{}
	for k, v := range br.Parameters {
		var val interface{}
		var use bool
		switch k {
		case "stp":
			val, use = validateBool(e, k, v)
		case "max-age":
			val, use = validateInt(e, k, v, 0, math.MaxInt8, 0)
		case "hello-time":
			val, use = validateInt(e, k, v, 0, math.MaxInt8, 0)
		case "forward-delay":
			val, use = validateInt(e, k, v, 0, math.MaxInt8, 0)
		case "ageing-time":
			val, use = validateInt(e, k, v, 0, math.MaxInt8, 0)
		case "priority":
			val, use = validateInt(e, k, v, 0, math.MaxUint16, 32768)
		default:
			e.Errorf("%s: Unknown parameter: %s", br.MatchID, k)
		}
		if use {
			params[k] = val
		}
	}
	if _, ok := params["stp"]; !ok {
		params["stp"] = true
	}
	br.Parameters = params
	return e.OrNil()
}

type Bond struct{ Interface }

func (b *Bond) Validate() error {
	b.Type = "bond"
	e := &err{prefix: b.Type + ":" + b.MatchID}
	e.Merge(b.Interface.Validate())
	params := map[string]interface{}{}
	for k, v := range b.Parameters {
		var res interface{}
		var use bool
		switch k {
		case "mode":
			res, use = validateStrIn(e, k, v,
				"balance-rr",
				"active-backup",
				"balance-xor",
				"broadcast",
				"802.3ad",
				"balance-tlb",
				"balance-alb", "")
		case "lacp-rate":
			res, use = validateStrIn(e, k, v, "fast", "slow", "")
		case "mii-monitor-interval":
			res, use = validateInt(e, k, v, 0, math.MaxInt8, 0)
		case "min-links":
			res, use = validateInt(e, k, v, 1, math.MaxInt8, 1)
		case "transmit-hash-policy":
			res, use = validateStrIn(e, k, v,
				"layer2",
				"layer3+4",
				"layer2+3",
				"encap2+3",
				"encap3+4", "")
		case "ad-select":
			res, use = validateStrIn(e, k, v,
				"stable",
				"bandwidth",
				"count")
		case "all-slaves-active":
			res, use = validateBool(e, k, v)
		case "arp-interval":
			res, use = validateInt(e, k, v, 0, math.MaxInt8, 0)
		case "arp-ip-targets":
			res, use = validateIPList(e, k, v, false)
		case "arp-validate":
			res, use = validateStrIn(e, k, v, "none", "active", "backup", "all")
		case "arp-all-targets":
			res, use = validateStrIn(e, k, v, "any", "all")
		case "up-delay":
			res, use = validateInt(e, k, v, 0, math.MaxInt8, 0)
		case "down-delay":
			res, use = validateInt(e, k, v, 0, math.MaxInt8, 0)
		case "fail-over-mac-policy":
			res, use = validateStrIn(e, k, v, "none", "active", "follow")
		case "gratuitious-arp":
			res, use = validateInt(e, k, v, 1, 127, 1)
		case "packets-per-slave":
			res, use = validateInt(e, k, v, 0, 65535, 1)
		case "resend-igmp":
			res, use = validateInt(e, k, v, 0, 255, 1)
		case "learn-packet-interval":
			res, use = validateInt(e, k, v, 1, 0x7fffffff, 1)
		case "primary":
			res, use = validateStrIn(e, k, v)
		default:
			e.Errorf("%s: Unknown parameter %s", b.MatchID, k)
		}
		if use {
			params[k] = res
		}
	}
	b.Parameters = params
	return e.OrNil()
}

type Vlan struct {
	Interface
	ID   int    `json:"id"`
	Link string `json:"link"`
}

func (v *Vlan) Validate() error {
	v.Type = "vlan"
	e := &err{prefix: v.Type + ":" + v.MatchID}
	e.Merge(v.Interface.Validate())
	validateInt(e, "id", v.ID, 0, 4094, 0)
	v.Parameters["id"] = v.ID
	v.Interfaces = []string{v.Link}
	return e.OrNil()
}

type Netplan struct {
	Network struct {
		Version   int64               `json:"version"`
		Renderer  string              `json:"renderer"`
		Ethernets map[string]Physical `json:"ethernets"`
		Bridges   map[string]Bridge   `json:"bridges"`
		Bonds     map[string]Bond     `json:"bonds"`
		Vlans     map[string]Vlan     `json:"vlans"`
	} `json:"network"`
}

func (n *Netplan) Read(src string) (*Layout, error) {
	log.Printf("Reading netplan from %s", src)
	in := os.Stdin
	if src != "" {
		i, e := os.Open(src)
		if e != nil {
			return nil, e
		}
		defer i.Close()
		in = i
	}
	buf, err := ioutil.ReadAll(in)
	if err != nil {
		return nil, err
	}
	if err := yaml.Unmarshal(buf, n); err != nil {
		return nil, err
	}
	return n.Compile()
}

type Layout struct {
	Renderer     string
	Interfaces   map[string]Interface
	Child2Parent map[string]string
	Roots        []string
}

func (l *Layout) Interface(name string) (res Interface, found bool) {
	res, found = l.Interfaces[name]
	if found {
		return
	}
	for _, phy := range phys {
		if name == phy.Name || name == phy.StableName {
			log.Printf("Adding unspecified phy %s", name)
			l.Interfaces[name] = Interface{
				MatchID:    phy.Name,
				Type:       "physical",
				HwAddr:     phy.HwAddr,
				Parameters: map[string]interface{}{},
			}
			claimedPhys[name] = l.Interfaces[name]
			res, found = l.Interfaces[name]
			break
		}
	}
	log.Printf("No matching interface to be auto-added for %s", name)
	return
}

func (l *Layout) Read(src string) (*Layout, error) {
	in := os.Stdin
	if src != "" {
		i, e := os.Open(src)
		if e != nil {
			return nil, e
		}
		defer i.Close()
		in = i
	}
	buf, err := ioutil.ReadAll(in)
	if err != nil {
		return nil, err
	}
	return l, yaml.Unmarshal(buf, l)
}

func (l *Layout) Write(dest string) error {
	out := os.Stdout
	if dest != "" {
		o, e := os.Create(dest)
		if e != nil {
			return e
		}
		defer o.Close()
	}
	buf, err := yaml.Marshal(l)
	if err != nil {
		return err
	}
	_, err = out.Write(buf)
	return err
}

func (l *Layout) Validate() error {
	log.Printf("Validating layout")
	e := &err{prefix: "layout"}
	l.Roots = []string{}
	members := []string{}
	for k := range l.Interfaces {
		members = append(members, k)
	}
	for _, k := range members {
		v, _ := l.Interface(k)
		log.Printf("Examining interface %s:%v", k, v)
		switch v.Type {
		case "physical":
			if len(v.HwAddr) == 0 {
				e.Errorf("Physical %s does not have a hardware address to use", k)
			}
			if len(v.Interfaces) != 0 {
				e.Errorf("Physical %s refers to sub interfaces", k)
			}
		case "bridge", "bond":
			for _, sub := range v.Interfaces {
				intf, ok := l.Interface(sub)
				if !ok {
					e.Errorf("%s:%s refers to unspecified network interface %s", v.Type, k, sub)
					continue
				}
				switch v.Type {
				case "bridge":
					if p, ok := l.Child2Parent[sub]; ok {
						e.Errorf("Bridge %s member %s already a member of %s:%s", k, sub, intf.Type, p)
					} else if intf.Configure() {
						e.Errorf("Bridge %s member %s wants to setup IP networking", k, sub)
					}
				case "bond":
					if intf.Type != "physical" {
						e.Errorf("Bond %s wants non-physical network interface %s(%s)", k, sub, intf.Type)
					} else if p, ok := l.Child2Parent[sub]; ok {
						e.Errorf("Bond member %s already a member of %s:%s", sub, l.Interfaces[p].Type, p)
					} else if intf.Configure() {
						e.Errorf("Bond %s member %s wants to setup IP networking", k, sub)
					}
				}
				l.Child2Parent[sub] = k
			}
		case "vlan":
			if len(v.Interfaces) != 1 {
				e.Errorf("Vlan %s must refer to exactly one other interface, not %d", k, len(v.Interfaces))
			} else {
				sub := v.Interfaces[0]
				intf, ok := l.Interface(sub)
				if !ok {
					e.Errorf("%s:%s refers to unspecified network interface %s", v.Type, k, sub)
				} else if p, ok := l.Child2Parent[sub]; !ok && l.Interfaces[p].Type != "vlan" {
					e.Errorf("Vlan %s parent interface %s already a member of %s:%s", k, sub, l.Interfaces[p].Type, p)
				} else if intf.Type == "vlan" {
					e.Errorf("Vlan %s parent interface %s cannot also be a vlan!", k, sub)
				} else {
					l.Child2Parent[sub] = k
				}
			}
		default:
			e.Errorf("Cannot handle interface %s:%s", v.Type, k)
		}
	}
	if !e.Empty() {
		return e
	}
	cleanInterfaces := map[string]struct{}{}
	cyclic := func(intf string) {
		next := intf
		cycle := false
		working := []string{}
		for {
			if _, ok := cleanInterfaces[next]; ok {
				cycle = false
				break
			}
			working = append(working, next)
			if next, cycle = l.Child2Parent[next]; cycle {
				for _, i := range working {
					cycle = i == next
					if cycle {
						break
					}
				}
			}
			if cycle {
				break
			}
		}
		if cycle {
			e.Errorf("Interface %s is in a cycle: %v", intf, working)
		}
		for _, i := range working {
			cleanInterfaces[i] = struct{}{}
		}
	}
	for k := range l.Interfaces {
		cyclic(k)
		if _, ok := l.Child2Parent[k]; !ok {
			l.Roots = append(l.Roots, k)
		}
	}
	sort.Strings(l.Roots)
	return e.OrNil()
}

func (n *Netplan) Compile() (*Layout, error) {
	log.Printf("Compiling netplan to layout")
	e := &err{prefix: "netplan"}
	l := &Layout{
		Renderer:     n.Network.Renderer,
		Interfaces:   map[string]Interface{},
		Child2Parent: map[string]string{},
	}
	log.Printf("Performing basic validation")
	validateInt(e, "version", n.Network.Version, 2, 2, 0)
	validateStrIn(e, "renderer", n.Network.Renderer, "networkd", "")
	if !e.Empty() {
		return nil, e
	}
	l.Interfaces = map[string]Interface{}
	// Make sure we always have something to operate on
	if n.Network.Ethernets == nil {
		n.Network.Ethernets = map[string]Physical{}
	}
	if n.Network.Bridges == nil {
		n.Network.Bridges = map[string]Bridge{}
	}
	if n.Network.Bonds == nil {
		n.Network.Bonds = map[string]Bond{}
	}
	if n.Network.Vlans == nil {
		n.Network.Vlans = map[string]Vlan{}
	}
	// Keep track of all known tags
	addOther := func(k string, intf Interface) {
		if other, ok := l.Interfaces[k]; ok {
			e.Errorf("Duplicate network definition! %s also defined in %s", k, other.Type)
		} else {
			log.Printf("Recording interface %s:%v", k, intf)
			l.Interfaces[k] = intf
		}
	}
	matchChildren := map[string][]string{}
	realSubs := func(s []string) []string {
		res := []string{}
		for _, v := range s {
			if ss, ok := matchChildren[v]; ok {
				res = append(res, ss...)
			} else {
				res = append(res, v)
			}
		}
		sort.Strings(res)
		return res
	}
	for k, v := range n.Network.Ethernets {
		v.Type = "physical"
		if v.MatchEmpty() {
			v.Match.Name = k
		}
		v.MatchID = k
		e.Merge(v.Validate())
		matchChildren[k] = v.Interfaces
		for _, n := range v.Interfaces {
			intf := v.Interface
			intf.Interfaces = []string{}
			intf.MatchID = n
			addOther(k, intf)
		}
	}
	if !e.Empty() {
		return nil, e
	}
	for k, v := range n.Network.Bridges {
		v.Type = "bridge"
		v.Interfaces = realSubs(v.Interfaces)
		addOther(k, v.Interface)
		e.Merge(v.Validate())
	}
	for k, v := range n.Network.Bonds {
		v.Type = "bond"
		v.Interfaces = realSubs(v.Interfaces)
		addOther(k, v.Interface)
		e.Merge(v.Validate())
	}
	for k, v := range n.Network.Vlans {
		v.Type = "vlan"
		v.Interfaces = realSubs(v.Interfaces)
		addOther(k, v.Interface)
		e.Merge(v.Validate())
	}
	e.Merge(l.Validate())
	return l, e.OrNil()
}
