// Package rhel implements support for writing out Redhat network
// config files in /etc/sysconfig/network-scripts/ifcfg-*
package rhel

import (
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/rackn/netwrangler/util"
)

// Rhel holds internal information needed to write
// out any required ifcfg-* and route-* files needed.
type Rhel struct {
	*util.Layout
	bindMacs        bool
	dest, finalDest string
}

func (r *Rhel) BindMacs() {
	r.bindMacs = true
}

func New(l *util.Layout) *Rhel {
	return &Rhel{Layout: l}
}

func (r *Rhel) writeOut(i util.Interface, e *util.Err) {
	ifcfgPath := path.Join(r.dest, "ifcfg-"+i.Name)
	ifcfg, err := os.Create(ifcfgPath)
	if err != nil {
		e.Errorf("Error creating %s: %v", ifcfgPath, err)
	}
	defer ifcfg.Close()
	writeKey := func(k string, v interface{}) {
		fmt.Fprintf(ifcfg, `%s="%v"
`, k, v)
	}
	fmt.Fprintf(ifcfg, "# Created by netwrangler\n")
	writeKey("DEVICE", i.Name)
	switch i.Type {
	case "bridge":
		writeKey("TYPE", "Bridge")
		if v, ok := i.Parameters["stp"]; ok {
			if v.(bool) {
				writeKey("STP", "yes")
				if vv, ok := i.Parameters["forward-delay"]; ok {
					delay := vv.(int)
					writeKey("DELAY", delay)
				}
			} else {
				writeKey("STP", "no")
			}
		}
	case "bond":
		bondopts := []string{}
		for k, v := range i.Parameters {
			key := strings.Replace(k, "-", "_", -1)
			switch key {
			case "all_slaves_active":
				if v.(bool) {
					v = "1"
				} else {
					v = "0"
				}
			case "arp_all_targets":
				if v.(bool) {
					v = "1"
				} else {
					v = "0"
				}
			case "arp_ip_targets":
				key = "arp_ip_target"
				val := v.([]net.IP)
				vals := []string{}
				for _, ip := range val {
					vals = append(vals, ip.String())
				}
				v = strings.Join(vals, ",")
			case "down_delay":
				key = "downdelay"
			case "fail_over_mac_policy":
				key = "fail_over_mac"
			case "gratuitous_arp":
				key = "num_grat_arp"
			case "mii_monitor_interval":
				key = "miimon"
			case "primary_reselect_policy":
				key = "primary_reselect"
			case "transmit_hash_policy":
				key = "xmit_hash_policy"
			case "up_delay":
				key = "updelay"
			}
			bondopts = append(bondopts, fmt.Sprintf("%s=%v", key, v))
		}
		sort.Strings(bondopts)
		writeKey("BONDING_OPTS", strings.Join(bondopts, " "))
	case "vlan":
		writeKey("VLAN", "yes")
		writeKey("VID", i.Parameters["id"])
		writeKey("PHYSDEV", i.Interfaces[0])
	case "physical":
		writeKey("TYPE", "Ethernet")
		if r.bindMacs {
			writeKey("HWADDR", i.CurrentHwAddr.String())
		}
	}
	parents := r.Child2Parent[i.Name]
	if len(parents) > 0 {
		for _, pName := range parents {
			parent := r.Interfaces[pName]
			switch parent.Type {
			case "bridge":
				writeKey("BRIDGE", pName)
			case "bond":
				writeKey("MASTER", pName)
				writeKey("SLAVE", "yes")
			}
		}
	}
	if i.Optional {
		writeKey("ONBOOT", "no")
	} else {
		writeKey("ONBOOT", "yes")
	}
	nw := i.Network
	if !nw.Configure() {
		return
	}
	v4addrs, v6addrs := []*util.IP{}, []*util.IP{}
	if len(nw.Addresses) > 0 {
		for _, addr := range nw.Addresses {
			if addr.IP.To4() != nil {
				v4addrs = append(v4addrs, addr)
			} else {
				v6addrs = append(v6addrs, addr)
			}
		}
	}
	if nw.Dhcp4 {
		writeKey("BOOTPROTO", "dhcp")
	} else {
		writeKey("BOOTPROTO", "none")
	}
	if nw.Nameservers != nil && len(nw.Nameservers.Addresses) > 0 {
		for idx, addr := range nw.Nameservers.Addresses {
			if idx > 1 {
				break
			}
			writeKey(fmt.Sprintf("DNS%d", idx+1), addr.String())
		}
	}
	for idx, addr := range v4addrs {
		writeKey(fmt.Sprintf("IPADDR%d", idx), addr.IP.To4().String())
		writeKey(fmt.Sprintf("NETMASK%d", idx), net.IP(addr.Mask).To4().String())
	}
	if nw.Gateway4 != nil {
		writeKey("GATEWAY", nw.Gateway4.IP.String())
	}
	if len(v6addrs) > 0 || nw.Dhcp6 || nw.AcceptRa {
		writeKey("IPV6INIT", "yes")
	}
	if nw.AcceptRa {
		writeKey("IPV6_AUTOCONF", "yes")
	}
	if nw.Dhcp6 {
		writeKey("DHCPV6C", "yes")
	}
	if len(v6addrs) > 0 {
		writeKey("IPV6ADDR", v6addrs[0].String())
		if len(v6addrs) > 1 {
			v6addrs = v6addrs[1:]
			addrs := make([]string, len(v6addrs))
			for i := range v6addrs {
				addrs[i] = v6addrs[i].String()
			}
			writeKey("IPV6ADDR_SECONDARIES", strings.Join(addrs, ","))
		}
	}
	if nw.Gateway6 != nil {
		if nw.Routes == nil {
			nw.Routes = []util.Route{}
		}
		nw.Routes = append(nw.Routes, util.Route{
			Via: nw.Gateway6,
			To: &util.IP{
				IP:   net.IPv6zero,
				Mask: net.IPMask(net.IPv6zero),
			},
		})
	}
	if len(nw.Routes) > 0 {
		routecfgPath := path.Join(r.dest, "route-"+i.Name)
		routecfg, err := os.Create(routecfgPath)
		if err != nil {
			e.Errorf("Error creating %s: %v", routecfgPath, err)
		}
		defer routecfg.Close()
		for idx := range nw.Routes {
			fmt.Fprintln(routecfg, nw.Routes[idx].IPString(i))
		}
	}
	if len(nw.RoutingPolicy) > 0 {
		rules4, rules6 := []util.RoutePolicy{}, []util.RoutePolicy{}
		for idx := range nw.RoutingPolicy {
			rule := nw.RoutingPolicy[idx]
			if rule.From == nil && rule.To == nil {
				continue
			}
			if rule.From != nil {
				if rule.From.IP.To4() == nil {
					rules6 = append(rules6, rule)
				} else {
					rules4 = append(rules4, rule)
				}
			} else {
				if rule.To.IP.To4() == nil {
					rules6 = append(rules6, rule)
				} else {
					rules4 = append(rules4, rule)
				}
			}
		}
		if len(rules4) > 0 {
			rulecfgPath := path.Join(r.dest, "rule-"+i.Name)
			rulecfg, err := os.Create(rulecfgPath)
			if err != nil {
				e.Errorf("Error creating %s: %v", rulecfgPath, err)
			}
			defer rulecfg.Close()
			for idx := range rules4 {
				fmt.Fprintln(rulecfg, rules4[idx].IPString())
			}
		}
		if len(rules6) > 0 {
			rulecfgPath := path.Join(r.dest, "rule6-"+i.Name)
			rulecfg, err := os.Create(rulecfgPath)
			if err != nil {
				e.Errorf("Error creating %s: %v", rulecfgPath, err)
			}
			defer rulecfg.Close()
			for idx := range rules6 {
				fmt.Fprintln(rulecfg, rules6[idx].IPString())
			}
		}
	}
}

func (r *Rhel) Write(dest string) error {
	tmp, err := ioutil.TempDir("", "netwrangler-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)
	e := &util.Err{Prefix: "rhel"}
	r.finalDest = dest
	r.dest = tmp
	for _, k := range r.Interfaces {
		r.writeOut(k, e)
	}
	if !e.Empty() {
		return e
	}
	toRemove := []string{}
	for _, glob := range []string{"ifcfg-*", "route-*", "rule-*", "rule6-*"} {
		names, err := filepath.Glob(path.Join(r.finalDest, glob))
		if err != nil {
			e.Merge(err)
			return e
		}
		toRemove = append(toRemove, names...)
	}
	for _, name := range toRemove {
		base := path.Base(name)
		if strings.HasSuffix(base, "-lo") {
			continue
		}
		os.Remove(name)
	}
	util.Copy(r.dest, r.finalDest, e)
	return e.OrNil()
}
