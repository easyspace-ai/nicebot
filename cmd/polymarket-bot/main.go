package main

import (
	"os"

	"limitorderbot/internal/cli"
)

func main() {
	os.Exit(cli.Execute())
}
