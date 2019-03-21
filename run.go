package netwrangler

import (
	"fmt"
	"io/ioutil"

	yaml "github.com/ghodss/yaml"
	"github.com/rackn/netwrangler/netplan"
	"github.com/rackn/netwrangler/rhel"
	"github.com/rackn/netwrangler/systemd"
	"github.com/rackn/netwrangler/util"
)

var (
	// The input formats we accept.  internal is the intermediate format netwrangler uses.
	SrcFormats = []string{"netplan", "internal"}
	// The output formats we can handle.  internal is the intermediate format netwrangler uses.
	DestFormats = []string{"systemd", "rhel", "internal"}
)

// GatherPhys gathers the physical nics that the system knows about.
// It is currently only supported on Linux systems.
func GatherPhys() ([]util.Phy, error) {
	return util.GatherPhys()
}

// GatherPhysFromFile gathers the physical nic information from a saved file.
// This can be used for unit testing or buld offline operations.
func GatherPhysFromFile(src string) (phys []util.Phy, err error) {
	var buf []byte
	buf, err = ioutil.ReadFile(src)
	if err != nil {
		err = fmt.Errorf("Error reading phys: %v", err)
		return
	}
	if err = yaml.Unmarshal(buf, &phys); err != nil {
		err = fmt.Errorf("Error unmarshalling phys: %v", err)
	}
	return
}

// Compile transforms network configuration settings from srcLoc in
// srcFmt into destFmt at destLoc, using phys as the base physical
// interfaces to build on.  if bindMacs is true, the generated format
// will bind to interface MAC addresses (or other unique physical
// addresses), otherwise the interface names at srcLoc must match what
// is present on the system at the time netwrangler is run.
func Compile(phys []util.Phy, srcFmt, destFmt, srcLoc, destLoc string, bindMacs bool) error {
	var (
		layout *util.Layout
		err    error
		in     util.Reader
		out    util.Writer
	)
	switch srcFmt {
	case "netplan":
		in = &netplan.Netplan{}
	case "internal":
		in = layout
	default:
		return fmt.Errorf("Unknown input format %s", srcFmt)
	}
	layout, err = in.Read(srcLoc, phys)
	if err != nil {
		return fmt.Errorf("Error reading '%s': %v", srcFmt, err)
	}

	switch destFmt {
	case "internal":
		out = layout
	case "systemd":
		out = systemd.New(layout)
	case "rhel":
		out = rhel.New(layout)
	default:
		return fmt.Errorf("Unknown output format %s", destFmt)
	}
	if bindMacs {
		out.BindMacs()
	}
	err = out.Write(destLoc)
	if err != nil {
		return fmt.Errorf("Error writing '%s': %v", destFmt, err)
	}
	return nil
}
