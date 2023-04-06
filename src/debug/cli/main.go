package main

import (
	"os"

	"github.com/jessevdk/go-flags"

	"debug/cli/command"
)

func main() {
	cmd := command.Commands{}
	_, err := flags.Parse(&cmd)
	if err != nil {
		os.Exit(1)
	}
}
