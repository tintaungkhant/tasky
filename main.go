package main

import (
	"fmt"
	"os"

	"github.com/tintaungkhant/tasky/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "tasky:", err)
		os.Exit(1)
	}
}
