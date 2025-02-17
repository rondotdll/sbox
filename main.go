package main

import (
	"sbox/sbox"
)

func main() {
	info, err := sbox.FetchSystemInfo()
	if err != nil {
		panic(err)
	}
	println("Hello, world!")
	println("You're using " + info.Base + "!")
}
