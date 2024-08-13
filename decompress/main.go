package main

import (
	"flag"
	"log"
	"os"

	ctw "github.com/ColeWyeth/factored-ctw"
)

var depth = flag.Int("depth", 56, "depth of Context Tree Weighting")

func main() {
	flag.Parse()
	if err := ctw.Decompress(os.Stdout, os.Stdin, *depth); err != nil {
		log.Fatalf("%v", err)
	}
}
