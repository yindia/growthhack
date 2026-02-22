package main

import (
	"os"

	cli "growthhack/cmd"
)

func main() {
	if err := cli.NewRootCmd().Execute(); err != nil {
		os.Exit(1)
	}
}
