package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	ctw "github.com/ColeWyeth/factored-ctw"
)

var depth = flag.Int("depth", 56, "depth of Context Tree Weighting")

func main() {
	// like compressor, take in a file name
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

	name2 := flag.Arg(1)
	if name2 == "" {
		flag.Usage()
		os.Exit(1)
	}

	// read the file to an io.reader
	f1, err := os.Open(name)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error when reading input files\n")
		return
	}
	defer f1.Close()

	// read the file to an io.reader
	f2, err := os.Create(name2)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error when reading input files\n")
		return
	}
	defer f2.Close()

	if err := ctw.Decompress(f2, f1, *depth); err != nil {
		log.Fatalf("%v", err)
	}
}
