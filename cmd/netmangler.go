package main

import (
	"log"
	"os"

	"github.com/rackn/netwrangler"
)

func main() {
	if err := netwrangler.Run(os.Args...); err != nil {
		log.Fatal(err)
	}
}
