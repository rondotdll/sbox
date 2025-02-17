package main

import "os"

func main() {
	_, e := os.Open("non_existant.txt")
	if e != nil {
		os.Exit(-1)
	}

	os.Exit(0)
}
