package util

import (
	"bytes"

	gnet "github.com/rackn/gohai/plugins/net"
)

// Match is a utility struct for matching physical interfaces against
// interfaces present on the machine.
type Match struct {
	Name       string            `json:"name,omitempty"`
	MacAddress gnet.HardwareAddr `json:"macaddress,omitempty"`
	Driver     string            `json:"driver,omitempty"`
}

func MatchPhys(m Match, tmpl Interface, phys []gnet.Interface) []Interface {
	res := []Interface{}
	for _, phyInt := range phys {
		if m.Name != "" &&
			!Glob2RE(m.Name).MatchString(phyInt.Name) &&
			!Glob2RE(m.Name).MatchString(phyInt.StableName) &&
			!Glob2RE(m.Name).MatchString(phyInt.OrdinalName) {
			continue
		}
		if m.Driver != "" && m.Driver != phyInt.Driver {
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
	return res
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
		if intf.Sys.IsPhysical {
			res = append(res, intf)
		}
	}
	return res, nil
}
