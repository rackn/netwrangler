// Package systemd implements support for writing out a
// systemd-networkd compatible set of network config files.
package systemd

import (
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"path"
	"path/filepath"
	"strings"

	gnet "github.com/rackn/gohai/plugins/net"
	"github.com/rackn/netwrangler/util"
)

// Systemd holds internal information needed to write out
// the appropriate .network, .netdev, and .link files
// that can be used to instantiate a network layout.
type Systemd struct {
	*util.Layout
	bindMacs        bool
	written         map[string]struct{}
	ctr             int
	dest, finalDest string
}

// BindMacs forces all Match sections for physical interfaces to match
// by MAC address.
func (s *Systemd) BindMacs() {
	s.bindMacs = true
}

func (s *Systemd) pathFor(ctr int, name, ext string) string {
	fName := fmt.Sprintf("%02d-%s.%s", ctr, name, ext)
	return path.Join(s.dest, fName)
}

func (s *Systemd) create(ctr int, intf util.Interface, e *util.Err) (io.WriteCloser, io.WriteCloser) {
	var nName, lName string
	nName = s.pathFor(ctr, intf.Name, "network")
	if intf.Type == "physical" {
		lName = s.pathFor(ctr, intf.Name, "link")
	} else {
		lName = s.pathFor(ctr, intf.Name, "netdev")
	}
	nf, nErr := os.Create(path.Join(nName))
	lf, lErr := os.Create(path.Join(lName))
	if nErr == nil && lErr == nil {
		return nf, lf
	}
	if nErr != nil {
		e.Errorf("Error creating %s: %v", path.Join(s.dest, nName), nErr)
	} else {
		nf.Close()
	}
	if lErr != nil {
		e.Errorf("Error creating %s: %v", path.Join(s.dest, lName), lErr)
	} else {
		lf.Close()
	}
	return nil, nil
}

// New returns a new Systemd for l.
func New(l *util.Layout) *Systemd {
	return &Systemd{
		written: map[string]struct{}{},
		Layout:  l,
		ctr:     60,
	}
}

func (s *Systemd) writeParents(i util.Interface, e *util.Err, nw io.Writer) {
	parents, ok := s.Child2Parent[i.Name]
	if !ok {
		return
	}
	// This link is owned by potentially many something elses.
	for _, pName := range parents {
		parent := s.Interfaces[pName]
		switch parent.Type {
		case "bridge":
			fmt.Fprintf(nw, "Bridge=%s\n", parent.Name)
		case "bond":
			fmt.Fprintf(nw, "Bond=%s\n", parent.Name)
			if pv, pok := parent.Parameters["primary"]; pok && pv.(string) == i.Name {
				fmt.Fprintf(nw, "PrimarySlave=%s\n", i.Name)
			}
		case "vlan":
			fmt.Fprintf(nw, "VLAN=%s\n", parent.Name)
		default:
			e.Errorf("%s:%s: No idea how to handle parent reference for %s:%s", i.Type, i.Name, parent.Type, parent.Name)
		}
	}
}

func (s *Systemd) writePhy(i util.Interface, e *util.Err, link io.Writer) {
	// Link file
	if v, ok := i.Parameters["wakeonlan"]; ok && v.(bool) {
		fmt.Fprintf(link, `[Match]
MACAddress=%s

[Link]
MACAddressPolicy=persistent
WakeOnLan=magic
`, i.CurrentHwAddr)
	}
}

func s2s(sep string) func(interface{}) interface{} {
	return func(i interface{}) interface{} {
		res := ""
		switch v := i.(type) {
		case []string:
			if len(v) > 0 {
				res = strings.Join(v, sep)
			}
		case []*gnet.IPNet:
			if len(v) > 0 {
				ips := make([]string, len(v))
				for i := range v {
					ips[i] = v[i].String()
				}
				res = strings.Join(ips, sep)
			}
		default:
			log.Panicf("s2s: cannot handle %v", i)
		}
		return res
	}
}

func writeParams(f io.Writer,
	e *util.Err,
	params []string, checks map[string]*util.Check, paramVals map[string]interface{}) {
	for _, k := range params {
		val, ok := paramVals[k]
		if !ok {
			continue
		}
		key := checks[k].Key(k)
		nv, valid := checks[k].Validate(e, k, val)
		if valid {
			fmt.Fprintf(f, "%s=%v\n", key, nv)
		}
	}
}

func (s *Systemd) writeBond(i util.Interface, e *util.Err, link io.Writer) {
	fmt.Fprintf(link, `[NetDev]
Name=%s
Kind=bond

[Bond]
`, i.Name)
	writeParams(link,
		e,
		[]string{
			"mode",
			"transmit-hash-policy",
			"lacp-rate",
			"mii-monitor-interval",
			"min-links",
			"ad-select",
			"all-slaves-active",
			"arp-interval",
			"arp-ip-targets",
			"arp-validate",
			"arp-all-targets",
			"up-delay",
			"down-delay",
			"fail-over-mac-policy",
			"gratuitous-arp",
			"packets-per-slave",
			"primary-reselect-policy",
			"resend-igmp",
			"learn-packet-interval",
		},
		map[string]*util.Check{
			"mode":                    util.X().D("balance-rr").K("Mode"),
			"transmit-hash-policy":    util.X().D("layer2").K("TransmitHashPolicy"),
			"lacp-rate":               util.X().D("slow").K("LacpTransmitRate"),
			"mii-monitor-interval":    util.X().D(0).K("MiiMonitorSec"),
			"min-links":               util.X().K("MinLinks"),
			"ad-select":               util.X().K("AdSelect"),
			"all-slaves-active":       util.X().K("AllSlavesActive"),
			"arp-interval":            util.X().K("ARPIntervalSec"),
			"arp-ip-targets":          util.X().K("ARPIPTargets").V(s2s(",")),
			"arp-validate":            util.X().K("ARPValidate"),
			"arp-all-targets":         util.X().K("ARPAllTargets"),
			"up-delay":                util.X().K("UpDelaySec"),
			"down-delay":              util.X().K("DownDelaySec"),
			"fail-over-mac-policy":    util.X().K("FailOverMACPolicy"),
			"gratuitous-arp":          util.X().K("GratuitousARP"),
			"packets-per-slave":       util.X().K("PacketsPerSlave"),
			"primary-reselect-policy": util.X().K("PrimaryReselectPolicy"),
			"resend-igmp":             util.X().K("ResendIGMP"),
			"learn-packet-interval":   util.X().K("LearnPacketIntervalSec"),
		},
		i.Parameters)
}

func (s *Systemd) writeBridge(i util.Interface, e *util.Err, link io.Writer) {
	fmt.Fprintf(link, `[NetDev]
Name=%s
Kind=bridge

[Bridge]
`, i.Name)
	writeParams(link,
		e,
		[]string{
			"stp",
			"max-age",
			"hello-time",
			"forward-delay",
			"ageing-time",
			"priority",
		},
		map[string]*util.Check{
			"stp":           util.X().K("STP"),
			"max-age":       util.X().K("MaxAgeSec"),
			"hello-time":    util.X().K("HelloTimeSec"),
			"forward-delay": util.X().K("ForwardDelaySec"),
			"ageing-time":   util.X().K("AgeingTimeSec"),
			"priority":      util.X().K("Priority"),
		},
		i.Parameters)
}

func (s *Systemd) writeVlan(i util.Interface, e *util.Err, link io.Writer) {
	fmt.Fprintf(link, `[NetDev]
Name=%s
Kind=vlan

[VLAN]
Id=%v
`, i.Name, i.Parameters["id"])
}

func writeRoute(r util.Route, e *util.Err, nw io.Writer) {
	fmt.Fprintf(nw, "\n[Route]\n")
	if r.From != nil {
		fmt.Fprintf(nw, "Source=%s\n", r.From)
	}
	if r.To != nil {
		fmt.Fprintf(nw, "Destination=%s\n", r.To)
	}
	if r.Via != nil {
		fmt.Fprintf(nw, "Gateway=%s\n", r.Via)
	}
	if r.OnLink {
		fmt.Fprintf(nw, "GatewayOnLink=%v\n", r.OnLink)
	}
	if r.Metric != 0 {
		fmt.Fprintf(nw, "Metric=%d\n", r.Metric)
	}
	if r.Type != "" {
		fmt.Fprintf(nw, "Type=%s\n", r.Type)
	}
	if r.Scope != "" {
		fmt.Fprintf(nw, "Scope=%s\n", r.Scope)
	}
	if r.Table != 0 {
		fmt.Fprintf(nw, "Table=%d\n", r.Table)
	}
}

func writeRoutePolicy(r util.RoutePolicy, e *util.Err, nw io.Writer) {
	fmt.Fprintf(nw, "\n[RoutingPolicyRule]\n")
	if r.From != nil {
		fmt.Fprintf(nw, "From=%s\n", r.From)
	}
	if r.To != nil {
		fmt.Fprintf(nw, "To=%s\n", r.To)
	}
	if r.Table != 0 {
		fmt.Fprintf(nw, "Table=%d\n", r.Table)
	}
	if r.Priority != 0 {
		fmt.Fprintf(nw, "Priority=%d\n", r.Priority)
	}
	if r.FWMark != 0 {
		fmt.Fprintf(nw, "FirewallMark=%d\n", r.FWMark)
	}
	if r.TOS != 0 {
		fmt.Fprintf(nw, "TypeOfService=%d\n", r.TOS)
	}
}

func writeDHCPv4(o util.Overrides, e *util.Err, nw io.Writer) {
	fmt.Fprintf(nw, "\n[DHCPv4]\n")
	if o.SendHostname != false{
		fmt.Fprintf(nw, "SendHostname=%t\n", o.SendHostname)
	}
	if o.Hostname != "" {
		fmt.Fprintf(nw, "Hostname=%s\n", o.Hostname)
	}
	if o.UseDNS != false {
		fmt.Fprintf(nw, "UseDNS=%t\n", o.UseDNS)
	}
	if o.UseNTP != false {
		fmt.Fprintf(nw, "UseNTP=%t\n", o.UseNTP)
	}
	if o.UseMTU != false {
		fmt.Fprintf(nw, "UseMTU=%t\n", o.UseMTU)
	}
	if o.UseDomains != "" {
		fmt.Fprintf(nw, "UseDomains=%s\n", o.UseDomains)
	}
	if o.UseRoutes != false {
		fmt.Fprintf(nw, "UseRoutes=%t\n", o.UseRoutes)
	}
}

func writeNetwork(n *util.Network, e *util.Err, nw io.Writer) {
	if n == nil {
		return
	}
	toWrite := map[string][][]string{}
	wr := func(section, k string, v interface{}) {
		var vals [][]string
		var ok bool
		if vals, ok = toWrite[section]; !ok {
			vals = [][]string{}
		}
		nv := []string{k, fmt.Sprintf("%v", v)}
		vals = append(vals, nv)
		toWrite[section] = vals
	}

	if n.Dhcp4 && n.Dhcp6 {
		wr("Network", "DHCP", "yes")
	} else if n.Dhcp6 {
		wr("Network", "DHCP", "ipv6")
	} else if n.Dhcp4 {
		wr("Network", "DHCP", "ipv4")
	}

	if n.DhcpIdentifier != "" {
		wr("DHCP", "ClientIdentifier", n.DhcpIdentifier)
	}

	wr("Network", "IPv6AcceptRA", n.AcceptRa)

	for _, a := range n.Addresses {
		wr("Network", "Address", a)
	}

	if n.Gateway4 != nil {
		wr("Network", "Gateway4", n.Gateway4)
	}

	if n.Gateway6 != nil {
		wr("Network", "Gateway6", n.Gateway6)
	}

	if n.Nameservers != nil {
		for _, dns := range n.Nameservers.Addresses {
			wr("Network", "DNS", dns)
		}
		if len(n.Nameservers.Search) > 0 {
			wr("Network", "Domains", s2s(",")(n.Nameservers.Search))
		}
	}

	if netLines, ok := toWrite["Network"]; ok {
		for _, s := range netLines {
			fmt.Fprintf(nw, "%s=%s\n", s[0], s[1])
		}
	}

	if dhcpLines, ok := toWrite["DHCP"]; ok && len(dhcpLines) > 0 {
		fmt.Fprintf(nw, "\n[DHCP]\n")
		for _, s := range dhcpLines {
			fmt.Fprintf(nw, "%s=%s\n", s[0], s[1])
		}
	}

	for _, r := range n.Routes {
		writeRoute(r, e, nw)
	}
	for _, r := range n.RoutingPolicy {
		writeRoutePolicy(r, e, nw)
	}
	for _, o := range n.Dhcp4Overrides {
		writeDHCPv4(o, e, nw)
	}
}

func (s *Systemd) writeOut(i util.Interface, e *util.Err) {
	if _, ok := s.written[i.Name]; ok {
		return
	}
	s.written[i.Name] = struct{}{}
	nw, link := s.create(s.ctr, i, e)
	if nw == nil || link == nil {
		return
	}
	defer nw.Close()
	defer link.Close()
	// Write link stuff first
	switch i.Type {
	case "physical":
		s.writePhy(i, e, link)
	case "bond":
		s.writeBond(i, e, link)
	case "bridge":
		s.writeBridge(i, e, link)
	case "vlan":
		s.writeVlan(i, e, link)
	default:
		e.Errorf("Cannot write interface %s:%s", i.Type, i.Name)
	}
	// Network file
	fmt.Fprintf(nw, "[Match]\n")
	if s.bindMacs && i.Type == "physical" {
		fmt.Fprintf(nw, "MACAddress=%s\n", i.CurrentHwAddr)
	} else {
		fmt.Fprintf(nw, "Name=%s\n", i.Name)
	}
	if i.Optional || len(i.MacAddress) > 0 {
		fmt.Fprintf(nw, "\n[Link]\n")
		if i.Optional {
			fmt.Fprintf(nw, "RequiredForOnline=no\n")
		}
		if len(i.MacAddress) > 0 {
			fmt.Fprintf(nw, "MACAddress=%s\n", i.MacAddress)
		}
	}
	fmt.Fprintf(nw, "\n[Network]\n")
	s.writeParents(i, e, nw)
	writeNetwork(i.Network, e, nw)
	for _, subName := range i.Interfaces {
		sub := s.Interfaces[subName]
		s.writeOut(sub, e)
	}
}

// Write implements the util.Writer interface.  For Systemd, dest must
// refer to a directory where systemd network config files will
// reside.  Internally, Write saves everything to a temp directory
// first, and only if no errors occured replaces the config in dest
// with the freshyl rendered config.
func (s *Systemd) Write(dest string) error {
	tmp, err := ioutil.TempDir("", "netwrangler-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)
	e := &util.Err{Prefix: "systemd-networkd"}
	s.finalDest = dest
	s.dest = tmp
	for _, k := range s.Roots {
		s.writeOut(s.Interfaces[k], e)
	}
	if !e.Empty() {
		return e
	}
	os.MkdirAll(s.finalDest, 0755)
	names, err := filepath.Glob(path.Join(s.finalDest, "*"))
	if err != nil {
		e.Merge(err)
		return e
	}
	for _, name := range names {
		base := path.Base(name)
		if base == "." || base == ".." {
			continue
		}
		os.RemoveAll(name)
	}
	util.Copy(s.dest, s.finalDest, e)
	return e.OrNil()
}
