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
	"math"
	"os"
	"sort"

	yaml "github.com/ghodss/yaml"
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
		"dhcp6":           util.D(false, util.VB()),
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
	Intf  util.Interface
	Match util.Match `json:"match"`
	WOL   bool       `json:"wakeonlan"`
}

func (pi phy) MatchPhys(phys []util.Phy) []util.Interface {
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
		"mode": util.C(util.VS("balance-rr",
			"active-backup",
			"balance-xor",
			"broadcast",
			"802.3ad",
			"balance-tlb",
			"balance-alb")),
		"lacp-rate":            util.C(util.VS("fast", "slow")),
		"mii-monitor-interval": util.C(util.VI(0, math.MaxInt8)),
		"min-links":            util.C(util.VI(1, math.MaxInt8)),
		"transmit-hash-policy": util.C(util.VS("layer2",
			"layer3+4",
			"layer2+3",
			"encap2+3",
			"encap3+4")),
		"ad-select":               util.C(util.VS("stable", "bandwidth", "count")),
		"all-slaves-active":       util.C(util.VB()),
		"arp-interval":            util.C(util.VI(0, math.MaxInt8)),
		"arp-ip-targets":          util.C(util.VIPS(false)),
		"arp-validate":            util.C(util.VS("none", "active", "backup", "all")),
		"arp-all-targets":         util.C(util.VS("any", "all")),
		"up-delay":                util.C(util.VI(0, math.MaxInt8)),
		"down-delay":              util.C(util.VI(0, math.MaxInt8)),
		"fail-over-mac-policy":    util.C(util.VS("none", "active", "follow")),
		"gratuitious-arp":         util.C(util.VI(1, 127)),
		"packets-per-slave":       util.C(util.VI(0, 65535)),
		"primary-reselect-policy": util.C(util.VS("always", "better", "failure")),
		"resend-igmp":             util.C(util.VI(0, 255)),
		"learn-packet-interval":   util.C(util.VI(1, 0x7fffffff)),
		"primary":                 util.C(util.VS()),
	})
}

func vlan() util.Validator {
	checks := map[string]*util.Check{
		"macaddress": util.C(util.VMAC()),
		"link": util.C(util.VS()).K("interfaces").V(func(i interface{}) interface{} {
			return []string{i.(string)}
		}),
		"id": util.C(util.VI(0, 4094)).K("parameters").V(func(i interface{}) interface{} {
			return map[string]interface{}{"id": i.(int)}
		}),
	}
	return func(e *util.Err, k string, v interface{}) (interface{}, bool) {
		res := util.Interface{}
		res.Type = "vlan"
		return res, util.ValidateAndMarshal(e, v, checks, &res)
	}
}

type Netplan struct {
	Network struct {
		Version   int                    `json:"version"`
		Ethernets map[string]interface{} `json:"ethernets"`
		Bridges   map[string]interface{} `json:"bridges"`
		Bonds     map[string]interface{} `json:"bonds"`
		Vlans     map[string]interface{} `json:"vlans"`
		Wifis     map[string]interface{} `json:"wifis"`
	} `json:"network"`
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

func (n *Netplan) compile(phys []util.Phy) (*util.Layout, error) {
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
			for _, newIntf := range util.MatchPhys(util.Match{Name: k}, util.Interface{}, phys) {
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
		realInts := intf.MatchPhys(phys)
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
	return n.compile(phys)
}
