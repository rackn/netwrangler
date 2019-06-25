package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	yaml "github.com/ghodss/yaml"
	"github.com/rackn/netwrangler"
	"github.com/rackn/netwrangler/util"
)

func main() {
	op, inFmt, outFmt, src, dest, physIn, bootMac := "", "", "", "", "", "", ""
	bindMacs := false
	args := os.Args[:]
	fs := flag.NewFlagSet(args[0], flag.ContinueOnError)
	fs.StringVar(&op, "op", "",
		`Operation to perform.
"gather" gathers information about the physical nics on the system in a form that can be used later with the -phys option
"compile" translates the -in formatted network spec from -src to -out formatted data at -dest`)
	fs.StringVar(&inFmt, "in", netwrangler.SrcFormats[0],
		fmt.Sprintf("Format to expect for input. Options: %v", strings.Join(netwrangler.SrcFormats, ", ")))
	fs.StringVar(&outFmt, "out", netwrangler.DestFormats[0],
		fmt.Sprintf("Format to render input to.  Options: %v", strings.Join(netwrangler.DestFormats, ", ")))
	fs.StringVar(&src, "src", "", "Location to get input from.  Defaults to stdin.")
	fs.StringVar(&dest, "dest", "", "Location to write output to.  Defaults to stdout.")
	fs.StringVar(&physIn, "phys", "", "File to read to gather current physical nics.  Defaults to reading them from the kernel.")
	fs.StringVar(&bootMac, "bootmac", "", "Mac address of the nic the system booted from.  Required for magic bootif name matching")
	fs.BoolVar(&bindMacs, "bindMacs", false, "Whether to write configs that force matching physical devices on MAC address")
	if err := fs.Parse(args[1:]); err != nil {
		log.Fatal(err)
	}
	switch op {
	case "gather":
		netwrangler.BootMac(bootMac)
		phys, err := netwrangler.GatherPhys()
		if err != nil {
			log.Fatal(err)
		}
		buf, err := yaml.Marshal(phys)
		if err != nil {
			log.Fatalf("Error marshalling phys: %v", err)
		}
		out := os.Stdout
		if dest != "" {
			out, err = os.Create(dest)
			if err != nil {
				log.Fatalf("Error opening dest: %v", err)
			}
			defer out.Close()
		}
		if _, err := out.Write(buf); err != nil {
			log.Fatalf("Error saving phys: %v", err)
		}
	case "compile":
		var (
			phys []util.Phy
			err  error
		)
		netwrangler.BootMac(bootMac)
		if physIn == "" {
			phys, err = netwrangler.GatherPhys()
		} else {
			phys, err = netwrangler.GatherPhysFromFile(physIn)
		}
		if err != nil {
			log.Fatalf("Error reading phys: %v", err)
		}
		err = netwrangler.Compile(phys, inFmt, outFmt, src, dest, bindMacs)
		if err != nil {
			log.Fatal(err)
		}
	}
}
