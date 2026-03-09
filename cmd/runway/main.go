package main

import (
	"fmt"
	"os"

	"github.com/Reeteshrajesh/runway/internal/cli"
)

// version is set at build time via -ldflags "-X main.version=x.y.z"
var version = "dev"

func main() {
	if err := cli.Run(os.Args[1:], version); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
