package netmangler

import (
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"

	yaml "github.com/ghodss/yaml"
)

type Phy struct {
	Name, StableName, Driver string
	HwAddr                   HardwareAddr
}

var (
	phys        []Phy
	claimedPhys map[string]Interface
	op          string
	inFmt       string
	outFmt      string
	src         string
	dest        string
	physIn      string
)

func parseArgs(args ...string) error {
	flag.StringVar(&op, "op", "",
		`Operation to perform.
"gather" gathers information about the physical nics on the system in a form that can be used later with the -phys option
"compile" translates the -in formatted network spec from -src to -out formatted data at -dest`)
	flag.StringVar(&inFmt, "in", "", "Format to expect for input.  `netplan` is the only option for now.")
	flag.StringVar(&outFmt, "out", "layout", "Format to render input to.  `layout` is the only option for now.")
	flag.StringVar(&src, "src", "", "Location to get input from.  Defaults to stdin.")
	flag.StringVar(&dest, "dest", "", "Location to write output to.  Defaults to stdout.")
	flag.StringVar(&physIn, "phys", "", "File to read to gather current physical nics.  Defaults to reading them from the kernel.")
	if len(args) > 0 {
		flag.CommandLine = flag.NewFlagSet("testing", flag.ContinueOnError)
		if err := flag.CommandLine.Parse(args); err != nil {
			return err
		}
	} else {
		flag.Parse()
	}
	return nil
}

func run() error {
	var netErr error
	claimedPhys = map[string]Interface{}
	if physIn == "" && phys == nil {
		phys, netErr = gatherPhys()
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
			out, err := os.Create(dest)
			if err != nil {
				return fmt.Errorf("Error opening dest: %v", err)
			}
			defer out.Close()
		}
		if _, err := out.Write(buf); err != nil {
			return fmt.Errorf("Error saving phys: %v", err)
		}
	case "compile":
		var layout *Layout
		var err error
		log.Printf("Compiling %s from %s", inFmt, src)
		switch inFmt {
		case "netplan":
			np := &Netplan{}
			layout, err = np.Read(src)
		case "layout":
			layout, err = layout.Read(src)
		default:
			return fmt.Errorf("Unknown input format %s", inFmt)
		}
		if err != nil {
			return fmt.Errorf("Error reading '%s': %v", inFmt, err)
		}
		log.Printf("Writing layout to %s:%s", outFmt, dest)
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

func Run(args ...string) error {
	if err := parseArgs(args...); err != nil {
		return err
	}
	return run()
}
