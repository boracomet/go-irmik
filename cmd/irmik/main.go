package main

import (
	"fmt"
	"os"

	"github.com/boracomet/go-irmik/internal/cli"
)

func main() {
	if err := cli.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "irmik: %v\n", err)
		os.Exit(1)
	}
}
