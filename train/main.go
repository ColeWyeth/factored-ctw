package main

import (
	"flag"
	// "log"
	"bufio"
	"encoding/json"
	"fmt"
	"os"

	ctw "github.com/ColeWyeth/factored-ctw"
)

var depth = flag.Int("depth", 56, "depth of Context Tree Weighting")

func train_model(name string, model *ctw.CTW) {

	// Borrowed from Encode function

	// Send file contents to model through the src channel.
	f, err := os.Open(name)
	if err != nil {
		fmt.Println(err)
	}
	defer f.Close()
	src := make(chan int)
	errc := make(chan error, 1)
	// We allow the reader to terminate early via a stopReader channel,
	// in case for example, a downstream error occured when writing to w.
	stopReader := make(chan struct{}, 1)
	go func() {
		defer close(src)
		errc <- func() error {
			scanner := bufio.NewScanner(f)
			scanner.Split(bufio.ScanBytes)
			for scanner.Scan() {
				var bt byte = scanner.Bytes()[0]
				for i := uint(0); i < 8; i++ {
					select {
					case src <- ((int(bt) & (1 << i)) >> i):
					case <-stopReader:
					}
				}
			}
			if err := scanner.Err(); err != nil {
				fmt.Println(err)
			}
			return nil
		}()
	}()

	for bit := range src {
		model.Observe(bit)
	}

	fmt.Printf("%e\n", model.Prob0())
	vom_model := ctw.ToVOM(model)

	// TODO: Dump the model as json
	f1, err := os.Create("model.json")
	if err != nil {
		fmt.Println("Error:", err)
		return
	}
	jsonBytes, err := json.Marshal(vom_model)
	if err != nil {
		fmt.Println("Error:", err)
		return
	}
	//fmt.Println(string(jsonBytes))
	f1.Write(jsonBytes)
	f1.Close()
}

func main() {
	flag.Parse()
	//model := ctw.NewFCTW(8, make([]int, *depth))
	model := ctw.NewCTW(make([]int, *depth))
	fmt.Printf("%e\n", model.Prob0())
	fname := "twelve.wav" //"0ab237b7-fb12-4687-afed-8d1e2070d621.wav" // "gettysburg.txt"
	train_model(fname, model)
}
