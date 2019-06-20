package util

import (
	"bytes"
	"net"
	"regexp"

	gnet "github.com/rackn/gohai/plugins/net"
)

// Match is a utility struct for matching physical interfaces against
// interfaces present on the machine.
type Match struct {
	Name       string            `json:"name,omitempty"`
	MacAddress gnet.HardwareAddr `json:"macaddress,omitempty"`
	Driver     string            `json:"driver,omitempty"`
}

func MatchPhys(m Match, tmpl Interface, phys []gnet.Interface) ([]Interface, error) {
	res := []Interface{}
	var matchName, matchDriver *regexp.Regexp
	var err error
	if m.Name != "" {
		matchName, err = Glob2RE(m.Name)
		if err != nil {
			return res, err
		}
	}
	if m.Driver != "" {
		matchDriver, err = Glob2RE(m.Driver)
		if err != nil {
			return res, err
		}
	}
	for _, phyInt := range phys {
		if matchName != nil &&
			!matchName.MatchString(phyInt.Name) &&
			!matchName.MatchString(phyInt.StableName) &&
			!matchName.MatchString(phyInt.OrdinalName) {
			continue
		}
		if matchDriver != nil && !matchDriver.MatchString(phyInt.Driver) {
			continue
		}
		if len(m.MacAddress) > 0 && !bytes.Equal(m.MacAddress, phyInt.HardwareAddr) {
			continue
		}
		intf := tmpl
		intf.Name = phyInt.Name
		intf.Type = "physical"
		intf.CurrentHwAddr = phyInt.HardwareAddr
		res = append(res, intf)
	}
	return res, nil
}

// GatherPhys gathers all the physical interfaces present on the machine.
// Loopback interfaces and virtual interfaces will be skipped.
func GatherPhys() ([]gnet.Interface, error) {
	info, err := gnet.Gather()
	if err != nil {
		return nil, err
	}
	res := []gnet.Interface{}

	for _, intf := range info.Interfaces {
		if intf.Sys.IsPhysical || intf.Flags&gnet.Flags(net.FlagLoopback) != 0 {
			res = append(res, intf)
		}
	}
	return res, nil
}
