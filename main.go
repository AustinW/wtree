package main

import (
	"os"

	"github.com/awhite/wtree/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}