package main

import (
	"lota/cli"
	"os"

	"github.com/fatih/color"
)

func main() {
	if err := cli.Run(); err != nil {
		color.Red("Error: %v\n", err)
		os.Exit(1)
	}
}
