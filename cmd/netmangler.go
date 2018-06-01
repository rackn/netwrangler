package main

import (
	"log"

	"github.com/digitalrebar/netmangler"
)

func main() {
	if err := netmangler.Run(); err != nil {
		log.Fatal(err)
	}
}
