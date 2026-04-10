package main

import (
	"fmt"
	"lota/cli"
	"os"
)

func main() {
	if err := cli.Run(); err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
}
