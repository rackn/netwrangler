package util

import (
	"bufio"
	"bytes"
	"net"
	"os"
	"os/exec"
	"path"
	"strings"
)

// Match is a utility struct for matching physical interfaces against
// interfaces present on the machine.
type Match struct {
	Name       string       `json:"name,omitempty"`
	MacAddress HardwareAddr `json:"macaddress,omitempty"`
	Driver     string       `json:"driver,omitempty"`
}

// Phy represents a physical interface that is actually present on the
// system being configured.
type Phy struct {
	Name, StableName, Driver string
	HwAddr                   HardwareAddr
}

func MatchPhys(m Match, tmpl Interface, phys []Phy) []Interface {
	res := []Interface{}
	for _, phyInt := range phys {
		if m.Name != "" &&
			!Glob2RE(m.Name).MatchString(phyInt.Name) &&
			!Glob2RE(m.Name).MatchString(phyInt.StableName) {
			continue
		}
		if m.Driver != "" && m.Driver != phyInt.Driver {
			continue
		}
		if len(m.MacAddress) > 0 && !bytes.Equal(m.MacAddress, phyInt.HwAddr) {
			continue
		}
		intf := tmpl
		intf.Name = phyInt.Name
		intf.Type = "physical"
		intf.CurrentHwAddr = phyInt.HwAddr
		res = append(res, intf)
	}
	return res
}

// GatherPhys gathers all the physical interfaces present on the machine.
// Loopback interfaces and virtual interfaces will be skipped.
func GatherPhys() ([]Phy, error) {
	baseifs, err := net.Interfaces()
	if err != nil {
		return nil, err
	}
	res := []Phy{}

	for _, intf := range baseifs {
		// Filter out virtual nics
		l, err := os.Readlink(path.Join("/sys/class/net", intf.Name))
		if err != nil {
			return nil, err
		}
		if strings.Contains(l, "/virtual/") {
			continue
		}
		phy := Phy{
			Name:       intf.Name,
			StableName: intf.Name,
			HwAddr:     HardwareAddr(intf.HardwareAddr),
		}
		// get StableName and Driver from udev
		cmd := exec.Command("udevadm", "info", "-q", "all", "-p", "/sys/class/net/"+phy.Name)
		buf := &bytes.Buffer{}
		cmd.Stdout = buf
		if err := cmd.Run(); err != nil {
			return nil, err
		}
		stableNameOrder := []string{"E: ID_NET_NAME_ONBOARD", "E: ID_NET_NAME_SLOT", "E: ID_NET_NAME_PATH"}
		stableNames := map[string]string{}
		sc := bufio.NewScanner(buf)
		for sc.Scan() {
			parts := strings.SplitN(sc.Text(), "=", 2)
			if len(parts) != 2 {
				continue
			}
			switch parts[0] {
			case "E: ID_NET_DRIVER":
				phy.Driver = parts[1]
			case "E: ID_NET_NAME_ONBOARD", "E: ID_NET_NAME_SLOT", "E: ID_NET_NAME_PATH":
				stableNames[parts[0]] = parts[1]
			}
		}
		for _, n := range stableNameOrder {
			if val, ok := stableNames[n]; ok {
				phy.StableName = val
				break
			}
		}
		res = append(res, phy)
	}
	return res, nil
}
