package netwrangler

import (
	"flag"
	"fmt"
	"io/ioutil"
	"os"

	yaml "github.com/ghodss/yaml"
	"github.com/rackn/netwrangler/netplan"
	"github.com/rackn/netwrangler/util"
)

var (
	phys        []util.Phy
	claimedPhys map[string]util.Interface
)

func Run(args ...string) error {
	op, inFmt, outFmt, src, dest, physIn := "", "", "", "", "", ""
	if len(args) == 0 {
		args = os.Args[:]
	}
	fs := flag.NewFlagSet(args[0], flag.ContinueOnError)
	fs.StringVar(&op, "op", "",
		`Operation to perform.
"gather" gathers information about the physical nics on the system in a form that can be used later with the -phys option
"compile" translates the -in formatted network spec from -src to -out formatted data at -dest`)
	fs.StringVar(&inFmt, "in", "", "Format to expect for input.  `netplan` is the only option for now.")
	fs.StringVar(&outFmt, "out", "layout", "Format to render input to.  `layout` is the only option for now.")
	fs.StringVar(&src, "src", "", "Location to get input from.  Defaults to stdin.")
	fs.StringVar(&dest, "dest", "", "Location to write output to.  Defaults to stdout.")
	fs.StringVar(&physIn, "phys", "", "File to read to gather current physical nics.  Defaults to reading them from the kernel.")
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
			err = layout.Write(dest)
		default:
			return fmt.Errorf("Unknown output format %s", outFmt)
		}
		if err != nil {
			return fmt.Errorf("Error writing '%s': %v", outFmt, err)
		}
	default:
		return fmt.Errorf("Unknown op `%s`.  Only `gather` and `compile` are supported", op)
	}
	return nil
}
