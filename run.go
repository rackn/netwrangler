package netwrangler

import (
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"strings"

	yaml "github.com/ghodss/yaml"
	"github.com/rackn/netwrangler/netplan"
	"github.com/rackn/netwrangler/rhel"
	"github.com/rackn/netwrangler/systemd"
	"github.com/rackn/netwrangler/util"
)

var (
	phys        []util.Phy
	claimedPhys map[string]util.Interface
	inFormats   = []string{"netplan", "layout"}
	outFormats  = []string{"systemd", "layout", "rhel"}
)

func Run(args ...string) error {
	op, inFmt, outFmt, src, dest, physIn := "", "", "", "", "", ""
	bindMacs := false
	if len(args) == 0 {
		args = os.Args[:]
	}
	fs := flag.NewFlagSet(args[0], flag.ContinueOnError)
	fs.StringVar(&op, "op", "",
		`Operation to perform.
"gather" gathers information about the physical nics on the system in a form that can be used later with the -phys option
"compile" translates the -in formatted network spec from -src to -out formatted data at -dest`)
	fs.StringVar(&inFmt, "in", inFormats[0], fmt.Sprintf("Format to expect for input. Options: %v", strings.Join(inFormats, ", ")))
	fs.StringVar(&outFmt, "out", outFormats[0],
		fmt.Sprintf("Format to render input to.  Options: %v", strings.Join(outFormats, ", ")))
	fs.StringVar(&src, "src", "", "Location to get input from.  Defaults to stdin.")
	fs.StringVar(&dest, "dest", "", "Location to write output to.  Defaults to stdout.")
	fs.StringVar(&physIn, "phys", "", "File to read to gather current physical nics.  Defaults to reading them from the kernel.")
	fs.BoolVar(&bindMacs, "bindMacs", false, "Whether to write configs that force matching physical devices on MAC address")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	var netErr error
	claimedPhys = map[string]util.Interface{}
	if physIn == "" {
		if phys == nil {
			phys, netErr = util.GatherPhys()
		}
	} else {
		buf, err := ioutil.ReadFile(physIn)
		if err != nil {
			return fmt.Errorf("Error reading phys: %v", err)
		}
		if err := yaml.Unmarshal(buf, &phys); err != nil {
			return fmt.Errorf("Error unmarshalling phys: %v", err)
		}
	}
	if netErr != nil {
		return fmt.Errorf("Error getting current network information: %v", netErr)
	}
	var out util.Writer
	switch op {
	case "gather":
		buf, err := yaml.Marshal(phys)
		if err != nil {
			return fmt.Errorf("Error marshalling phys: %v", err)
		}
		out := os.Stdout
		if dest != "" {
			out, err = os.Create(dest)
			if err != nil {
				return fmt.Errorf("Error opening dest: %v", err)
			}
			defer out.Close()
		}
		if _, err := out.Write(buf); err != nil {
			return fmt.Errorf("Error saving phys: %v", err)
		}
	case "compile":
		var layout *util.Layout
		var err error
		switch inFmt {
		case "netplan":
			np := &netplan.Netplan{}
			layout, err = np.Read(src, phys)
		case "layout":
			layout, err = layout.Read(src)
		default:
			return fmt.Errorf("Unknown input format %s", inFmt)
		}
		if err != nil {
			return fmt.Errorf("Error reading '%s': %v", inFmt, err)
		}

		switch outFmt {
		case "layout":
			out = layout
		case "systemd":
			out = systemd.New(layout)
		case "rhel":
			out = rhel.New(layout)
		default:
			return fmt.Errorf("Unknown output format %s", outFmt)
		}
		if bindMacs {
			out.BindMacs()
		}
		err = out.Write(dest)
		if err != nil {
			return fmt.Errorf("Error writing '%s': %v", outFmt, err)
		}
	default:
		return fmt.Errorf("Unknown op `%s`.  Only `gather` and `compile` are supported", op)
	}
	return nil
}
