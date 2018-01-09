package main

import (
	"debug/cli/command"
	"os"

	flags "github.com/jessevdk/go-flags"
)

func main() {
	cmd := command.Commands{}
	_, err := flags.Parse(&cmd)
	if err != nil {
		os.Exit(1)
	}
}
