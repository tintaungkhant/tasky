package main

import (
	"fmt"
	"os"

	"github.com/tintaungkhant/tasky/cmd"
	"github.com/tintaungkhant/tasky/internal/ui"
)

func main() {
	if err := cmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, ui.Errorf("tasky: %s", err))
		os.Exit(1)
	}
}
