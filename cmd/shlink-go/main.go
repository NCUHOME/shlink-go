package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/joho/godotenv"

	"github.com/hakimyu/shlink-go/internal/cli"
)

func main() {
	if err := godotenv.Load(); err != nil && !errors.Is(err, os.ErrNotExist) {
		fmt.Fprintf(os.Stderr, "warning: load .env: %v\n", err)
	}
	os.Exit(cli.Execute())
}
