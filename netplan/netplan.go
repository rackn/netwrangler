// Package netplan implements support for reading Netplan.io compatible
// yaml formatted config files.  Missing parts are:
//
// * This package does not care about or respect the hierarchical
//   configuration files that netplan.io supports.  netwrangler is
//   designed to run as part of initial system configuration (either
//   initial OS install or image deployment) with the network layout
//   for the system being fed to it from an external source
//   (dr-provision or some other provisioning engine).
//
// * There is no support for nic renaming or MAC address reassignment.
//   Support for these features may be added in a future release.
//
// * There is no support for wifi or for separate backend renderers on
//   a per-interface basis. The former may be added in a future
//   release, the latter is never likely to be added.
package netplan

import (
	"io/ioutil"
	"log"
	"math"
	"os"
	"sort"

	yaml "github.com/ghodss/yaml"
	gnet "github.com/rackn/gohai/plugins/net"
	"github.com/rackn/netwrangler/util"
)

func nameservers() util.Validator {
	checks := map[string]*util.Check{
		"search":    util.C(util.VSS()),
		"addresses": util.C(util.VIPS(false)),
	}
	return func(e *util.Err, k string, ns interface{}) (interface{}, bool) {
		res := &util.NSInfo{}
		resOK := util.ValidateAndMarshal(e, ns, checks, res)
		return res, resOK
	}
}

func routes() util.Validator {
	checks := map[string]*util.Check{
		"from":    util.C(util.VIP()),
		"to":      util.C(util.VIP()),
		"via":     util.C(util.VIP()),
		"on-link": util.C(util.VB()),
		"metric":  util.C(util.VI(0, math.MaxUint32)),
		"table":   util.C(util.VI(0, math.MaxUint32)),
		"scope":   util.C(util.VS("global", "link", "host")),
		"type":    util.D("unicast", util.VS("unicast", "unreachable", "blackhole", "prohibit")),
	}
	return func(e *util.Err, k string, v interface{}) (interface{}, bool) {
		res := []util.Route{}
		resOK := true
		ra, ok := v.([]interface{})
		if !ok {
			e.Errorf("routes in invalid format: %T", v)
			return res, false
		}
		for i, vv := range ra {
			route := util.Route{}
			if !util.ValidateAndMarshal(e, vv, checks, &route) {
				e.Errorf("Invalid route %d", i)
				resOK = false
				continue
			}
			res = append(res, route)
		}
		return res, resOK
	}
}

func routepolicy() util.Validator {
	checks := map[string]*util.Check{
		"from":     util.C(util.VIP()),
		"to":       util.C(util.VIP()),
		"table":    util.C(util.VI(0, math.MaxUint32)),
		"priority": util.C(util.VI(0, math.MaxUint32)),
		"mark":     util.C(util.VI(0, math.MaxUint8)),
		"tos":      util.C(util.VI(0, math.MaxUint8)),
	}
	return func(e *util.Err, k string, v interface{}) (interface{}, bool) {
		res := []util.RoutePolicy{}
		resOK := true
		ra, ok := v.([]interface{})
		if !ok {
			e.Errorf("routes in invalid format: %T", v)
			return res, false
		}
		for i, vv := range ra {
			routePolicy := util.RoutePolicy{}
			if !util.ValidateAndMarshal(e, vv, checks, &routePolicy) {
				e.Errorf("Invalid routing policy: %d", i)
				resOK = false
				continue
			}
			res = append(res, routePolicy)
		}
		return res, resOK
	}
}

func network() util.Validator {
	checks := map[string]*util.Check{
		"dhcp4":           util.D(false, util.VB()),
		"dhcp4-overrides": util.C(override()),
		"dhcp6":           util.D(false, util.VB()),
		"dhcp6-overrides": util.C(override()),
		"dhcp-identifier": util.C(util.VS()),
		"accept-ra":       util.D(true, util.VB()),
		"addresses":       util.C(util.VIPS(true)),
		"gateway4":        util.C(util.VIP4()),
		"gateway6":        util.C(util.VIP6()),
		"nameservers":     util.C(nameservers()),
		"routes":          util.C(routes()),
		"routing-policy":  util.C(routepolicy()),
	}
	return func(e *util.Err, k string, v interface{}) (interface{}, bool) {
		res := &util.Network{}
		resOK := util.ValidateAndMarshal(e, v, checks, res)
		return res, resOK
	}
}

func override() util.Validator {
	checks := map[string]*util.Check{
		"use-dns": util.D(true, util.VB()),
		"use-ntp": util.D(true, util.VB()),
		"send-hostname": util.D(true, util.VB()),
		"use-mtu": util.D(true, util.VB()),
		"hostname": util.C(util.VS()),
		"use-routes": util.D(true, util.VB()),
		"route-metric": util.C(util.VI(0, math.MaxUint32)),
		"use-domains": util.D(true, util.VS("true", "false", "route")),
	}
	return func(e *util.Err, k string, v interface{}) (interface{}, bool) {
		res := &util.Overrides{}
		resOK := util.ValidateAndMarshal(e, v, checks, res)
		return res, resOK
	}
}

func phymatch() util.Validator {
	checks := map[string]*util.Check{
		"name":       util.C(util.VS()),
		"macaddress": util.C(util.VMAC()),
		"driver":     util.C(util.VS()),
	}
	return func(e *util.Err, k string, v interface{}) (interface{}, bool) {
		res := util.Match{}
		resOK := util.ValidateAndMarshal(e, v, checks, &res)
		return res, resOK
	}
}

type phy struct {
	Intf     util.Interface
	Match    util.Match `json:"match"`
	WOL      bool       `json:"wakeonlan"`
	Optional bool       `json:"optional"`
}

func (pi phy) matchPhys(phys []util.Phy) ([]util.Interface, error) {
	if pi.Match.Name == "" &&
		pi.Match.Driver == "" &&
		len(pi.Match.MacAddress) == 0 {
		pi.Match.Name = pi.Intf.MatchID
	}
	return util.MatchPhys(pi.Match, pi.Intf, phys)
}

func ethernet() util.Validator {
	checks := map[string]*util.Check{
		"match":      util.C(phymatch()),
		"wakeonlan":  util.C(util.VB()),
		"set-name":   util.C(util.ValidateUnsupp),
		"macaddress": util.C(util.ValidateUnsupp),
		"optional":   util.C(util.VB()),
	}
	return func(e *util.Err, k string, v interface{}) (interface{}, bool) {
		res := phy{}
		res.Intf = util.NewInterface()
		if !util.ValidateAndMarshal(e, v, checks, &res) {
			e.Errorf("%T not castable to an ethernet interface", v)
			return res, false
		}
		nw, ok := network()(e, "network", v)
		if !ok {
			return res, false
		}
		res.Intf.Type = "physical"
		if res.WOL {
			res.Intf.Parameters["wakeonlan"] = res.WOL
		}
		res.Intf.Optional = res.Optional
		res.Intf.Network = nw.(*util.Network)
		return res, true
	}
}

func pValidate(checks map[string]*util.Check) util.Validator {
	return func(e *util.Err, k string, v interface{}) (interface{}, bool) {
		res := map[string]interface{}{}
		resOK := util.ValidateAndMarshal(e, v, checks, &res)
		return res, resOK
	}
}

func bb(kind string, pchecks map[string]*util.Check) util.Validator {
	checks := map[string]*util.Check{
		"macaddress": util.C(util.VMAC()),
		"interfaces": util.C(util.VSS()),
		"parameters": util.C(pValidate(pchecks)),
		"optional":   util.C(util.VB()),
	}
	return func(e *util.Err, k string, v interface{}) (interface{}, bool) {
		res := util.NewInterface()
		if !util.ValidateAndMarshal(e, v, checks, &res) {
			e.Errorf("%T not castable to a %s interface", v, kind)
			return res, false
		}
		nw, ok := network()(e, "network", v)
		if !ok {
			return res, false
		}
		res.Type = kind
		res.Network = nw.(*util.Network)
		return res, true
	}
}

func bridge() util.Validator {
	return bb("bridge", map[string]*util.Check{
		"stp":           util.D(true, util.VB()),
		"max-age":       util.C(util.VI(0, math.MaxInt8)),
		"hello-time":    util.C(util.VI(0, math.MaxInt8)),
		"forward-delay": util.C(util.VI(0, math.MaxInt8)),
		"ageing-time":   util.C(util.VI(0, math.MaxInt8)),
		"priority":      util.D(32768, util.VI(0, math.MaxInt16)),
	})
}

func bond() util.Validator {
	return bb("bond", map[string]*util.Check{
		"ad-select":               util.C(util.VS("stable", "bandwidth", "count")),
		"all-slaves-active":       util.C(util.VB()),
		"arp-all-targets":         util.C(util.VS("any", "all")),
		"arp-interval":            util.C(util.VI(0, math.MaxInt8)),
		"arp-ip-targets":          util.C(util.VIPS(false)),
		"arp-validate":            util.C(util.VS("none", "active", "backup", "all")),
		"down-delay":              util.C(util.VI(0, math.MaxInt8)),
		"fail-over-mac-policy":    util.C(util.VS("none", "active", "follow")),
		"gratuitous-arp":          util.C(util.VI(1, 127)),
		"lacp-rate":               util.C(util.VS("fast", "slow")),
		"learn-packet-interval":   util.C(util.VI(1, 0x7fffffff)),
		"mii-monitor-interval":    util.C(util.VI(0, math.MaxInt8)),
		"min-links":               util.C(util.VI(1, math.MaxInt8)),
		"mode":                    util.C(util.VS("balance-rr", "active-backup", "balance-xor", "broadcast", "802.3ad", "balance-tlb", "balance-alb")),
		"packets-per-slave":       util.C(util.VI(0, 65535)),
		"primary":                 util.C(util.VS()),
		"primary-reselect-policy": util.C(util.VS("always", "better", "failure")),
		"resend-igmp":             util.C(util.VI(0, 255)),
		"transmit-hash-policy":    util.C(util.VS("layer2", "layer3+4", "layer2+3", "encap2+3", "encap3+4")),
		"up-delay":                util.C(util.VI(0, math.MaxInt8)),
	})
}

func vlan() util.Validator {

	type li struct {
		L string `json:"link"`
		I int    `json:"id"`
	}
	checksI := map[string]*util.Check{
		"macaddress": util.C(util.VMAC()),
	}
	checksLI := map[string]*util.Check{
		"link": util.C(util.VS()),
		"id":   util.C(util.VI(0, 4094)),
	}
	return func(e *util.Err, k string, v interface{}) (interface{}, bool) {
		rres := &li{}
		rresOK := util.ValidateAndMarshal(e, v, checksLI, rres)
		res := util.NewInterface()
		res.Type = "vlan"
		resOK := util.ValidateAndMarshal(e, v, checksI, &res)
		res.Interfaces = []string{rres.L}
		res.Parameters["id"] = rres.I
		if nw, nwok := network()(e, "network", v); nwok {
			if nw != nil {
				network := nw.(*util.Network)
				if network.Configure() {
					res.Network = network
				}
			}
		} else {
			resOK = false
		}
		return res, (resOK && rresOK)
	}
}

// Netplan is the basic struct for netplan.io style network configs.
type Netplan struct {
	Network struct {
		Version   int                    `json:"version"`
		Renderer  string                 `json:"renderer,omitempty"`
		Ethernets map[string]interface{} `json:"ethernets,omitempty"`
		Bridges   map[string]interface{} `json:"bridges,omitempty"`
		Bonds     map[string]interface{} `json:"bonds,omitempty"`
		Vlans     map[string]interface{} `json:"vlans,omitempty"`
		Wifis     map[string]interface{} `json:"wifis,omitempty"`
	} `json:"network"`
	bindMac bool
}

func (n *Netplan) BindMacs() {
	n.bindMac = true
}

func getNames(i map[string]interface{}) []string {
	res := []string{}
	if i == nil {
		return res
	}
	for k := range i {
		res = append(res, k)
	}
	sort.Strings(res)
	return res
}

type Common struct {
	*util.Network
	MacAddress gnet.HardwareAddr `json:"macaddress,omitempty"`
	Renderer   string            `json:"renderer,omitempty"`
	Optional   bool              `json:"optional,omitempty"`
}

func asCommon(i util.Interface) Common {
	return Common{
		Network:    i.Network,
		Optional:   i.Optional,
		MacAddress: i.MacAddress,
	}
}

type Ether struct {
	Common
	Match     map[string]string `json:"match,omitempty"`
	WakeOnLan bool              `json:"wakeonlan,omitempty"`
}

func asEther(i util.Interface) Ether {
	res := Ether{Common: asCommon(i)}
	res.MacAddress = nil
	if v, ok := i.Parameters[`wakeonlan`]; ok && v.(bool) {
		res.WakeOnLan = true
	}
	res.Match = map[string]string{
		"macaddress": i.CurrentHwAddr.String(),
	}
	return res
}

type Bond struct {
	Common
	Interfaces []string               `json:"interfaces,omitempty"`
	Parameters map[string]interface{} `json:"parameters,omitempty"`
}

func asBond(i util.Interface) Bond {
	return Bond{
		Common:     asCommon(i),
		Parameters: i.Parameters,
		Interfaces: i.Interfaces,
	}
}

type Bridge struct {
	Common
	Interfaces []string               `json:"interfaces,omitempty"`
	Parameters map[string]interface{} `json:"parameters,omitempty"`
}

func asBridge(i util.Interface) Bridge {
	return Bridge{
		Common:     asCommon(i),
		Parameters: i.Parameters,
		Interfaces: i.Interfaces,
	}
}

type Vlan struct {
	Common
	ID   int    `json:"id"`
	Link string `json:"link"`
}

func asVlan(i util.Interface) Vlan {
	return Vlan{
		Common: asCommon(i),
		ID:     i.Parameters[`id`].(int),
		Link:   i.Interfaces[0],
	}
}

func (n *Netplan) Write(dest string) error {
	out := os.Stdout
	if dest != "" {
		o, e := os.Create(dest)
		if e != nil {
			return e
		}
		defer o.Close()
		out = o
	}
	toElide := []string{}
	for k := range n.Network.Ethernets {
		if !n.bindMac {
			delete(n.Network.Ethernets[k].(Ether).Match, "macaddress")
		}
		buf, err := yaml.Marshal(n.Network.Ethernets[k])
		if err == nil && string(buf) == "{}\n" {
			toElide = append(toElide, k)
		}
	}
	for _, k := range toElide {
		delete(n.Network.Ethernets, k)
	}
	buf, err := yaml.Marshal(n)
	if err != nil {
		return err
	}
	_, err = out.Write(buf)
	return err
}

// New creates a new Netplan that will render using networkd.
func New(l *util.Layout) *Netplan {
	res := &Netplan{}
	res.Network.Version = 2
	res.Network.Renderer = "networkd"
	res.Network.Ethernets = map[string]interface{}{}
	res.Network.Bridges = map[string]interface{}{}
	res.Network.Bonds = map[string]interface{}{}
	res.Network.Vlans = map[string]interface{}{}
	for _, i := range l.Interfaces {
		switch i.Type {
		case "physical":
			res.Network.Ethernets[i.Name] = asEther(i)
		case "bond":
			res.Network.Bonds[i.Name] = asBond(i)
		case "bridge":
			res.Network.Bridges[i.Name] = asBridge(i)
		case "vlan":
			res.Network.Vlans[i.Name] = asVlan(i)
		default:
			log.Panicf("Unknown interface type %s", i.Type)
		}
	}
	return res
}

func (n *Netplan) Compile(phys []util.Phy) (*util.Layout, error) {
	e := &util.Err{Prefix: "netplan"}
	l := &util.Layout{
		Interfaces: map[string]util.Interface{},
	}
	util.ValidateInt(e, "version", n.Network.Version, 2, 2)
	if n.Network.Wifis != nil {
		e.Errorf("Wifi interfaces not supported")
	}
	// Keep track of all known tags
	addOther := func(name, matchID string, intf util.Interface) {
		intf.Name = name
		intf.MatchID = matchID
		if other, ok := l.Interfaces[intf.Name]; ok {
			e.Errorf("Duplicate network definition! %s also defined in %s", intf.Name, other.Type)
		} else {
			l.Interfaces[name] = intf
		}
		for _, k := range intf.Interfaces {
			subs, err := util.MatchPhys(util.Match{Name: k}, util.Interface{}, phys)
			if err != nil {
				e.Errorf("Invalid interface match: %v", err)
				return
			}
			for _, newIntf := range subs {
				newIntf.MatchID = newIntf.Name
				if _, ok := l.Interfaces[newIntf.Name]; !ok {
					l.Interfaces[newIntf.Name] = newIntf
				}
			}
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
	for _, k := range getNames(n.Network.Ethernets) {
		nv, valid := ethernet()(e, "ethernet:"+k, n.Network.Ethernets[k])
		if !valid {
			continue
		}
		intf := nv.(phy)
		intf.Intf.MatchID = k
		realInts, err := intf.matchPhys(phys)
		if err != nil {
			e.Errorf("Invalid interface match: %v", err)
			continue
		}
		if len(realInts) == 0 {
			e.Errorf("Ethernet interface %s does not resolve to any interfaces", k)
			continue
		}
		intNames := []string{}
		for _, realInt := range realInts {
			intNames = append(intNames, realInt.Name)
			addOther(realInt.Name, k, realInt)
		}
		matchChildren[k] = intNames
	}
	for _, k := range getNames(n.Network.Bonds) {
		nv, valid := bond()(e, "bond:"+k, n.Network.Bonds[k])
		if valid {
			addOther(k, k, nv.(util.Interface))
		}
	}
	for _, k := range getNames(n.Network.Bridges) {
		nv, valid := bridge()(e, "bridge:"+k, n.Network.Bridges[k])
		if valid {
			addOther(k, k, nv.(util.Interface))
		}
	}
	for _, k := range getNames(n.Network.Vlans) {
		nv, valid := vlan()(e, "vlan:"+k, n.Network.Vlans[k])
		if valid {
			addOther(k, k, nv.(util.Interface))
		}
	}
	for k, v := range l.Interfaces {
		v.Interfaces = realSubs(v.Interfaces)
		l.Interfaces[k] = v
	}
	e.Merge(l.Validate())
	return l, e.OrNil()
}

// Read satisfies the Reader interface so that Netplan can be used as
// a input format.
func (n *Netplan) Read(src string, phys []util.Phy) (*util.Layout, error) {
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
	return n.Compile(phys)
}
