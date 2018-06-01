package netmangler

import (
	"bufio"
	"bytes"
	"net"
	"os"
	"os/exec"
	"path"
	"strings"
)

func gatherPhys() ([]Phy, error) {
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
