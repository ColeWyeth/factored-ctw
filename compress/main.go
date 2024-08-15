package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	ctw "github.com/ColeWyeth/factored-ctw"
)

var depth = flag.Int("depth", 56, "depth of Context Tree Weighting")
var verbose = flag.Bool("verbose", false, "verbosity")

func main() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "usage: %s [flags] sroucefilename targetfilename\n", os.Args[0])
		flag.PrintDefaults()
	}
	flag.Parse()
	name := flag.Arg(0)
	if name == "" {
		flag.Usage()
		os.Exit(1)
	}

	// a second path for compressed results
	name2 := flag.Arg(1)
	if name2 == "" {
		flag.Usage()
		os.Exit(1)
	}

	f, err := os.Create(name2)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error when reading input files\n")
		return
	}
	defer f.Close()

	if err := ctw.Compress(f, name, *depth); err != nil {
		log.Fatalf("%v", err)
	}
}
