package main

import (
	"os"

	"github.com/data219/huectl/internal/cli"
)

func main() {
	code := cli.Execute()
	os.Exit(code)
}
