package main

import (
	"os"

	"github.com/hakimyu/shlink-go/internal/cli"
)

func main() {
	os.Exit(cli.Execute())
}
