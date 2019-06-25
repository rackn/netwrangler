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

type Phy struct {
	gnet.Interface
	BootIf bool
}

func MatchPhys(m Match, tmpl Interface, phys []Phy) ([]Interface, error) {
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
	for _, phy := range phys {
		if matchDriver != nil && !matchDriver.MatchString(phy.Driver) {
			continue
		}
		if len(m.MacAddress) > 0 && !bytes.Equal(m.MacAddress, phy.HardwareAddr) {
			continue
		}
		if m.Name == "bootif" {
			if !phy.BootIf {
				continue
			}
		} else if matchName != nil && !(matchName.MatchString(phy.Name) ||
			matchName.MatchString(phy.StableName) ||
			matchName.MatchString(phy.OrdinalName)) {
			continue
		}
		intf := tmpl
		intf.Name = phy.Name
		intf.Type = "physical"
		intf.CurrentHwAddr = phy.HardwareAddr
		res = append(res, intf)
	}
	return res, nil
}

// GatherPhys gathers all the physical interfaces present on the machine.
// Loopback interfaces and virtual interfaces will be skipped.
func GatherPhys() ([]Phy, error) {
	info, err := gnet.Gather()
	if err != nil {
		return nil, err
	}
	res := []Phy{}

	for _, intf := range info.Interfaces {
		if intf.Sys.IsPhysical || intf.Flags&gnet.Flags(net.FlagLoopback) != 0 {
			res = append(res, Phy{intf, false})
		}
	}
	return res, nil
}
