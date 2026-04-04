package main

import (
	"fmt"
	"os"

	"github.com/khaledmoayad/clawgo/internal/cli"
)

func main() {
	cmd := cli.NewRootCmd(cli.Version)
	if err := cmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
