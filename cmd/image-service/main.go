package main

import (
	"os"

	"github.com/jlentink/image-service/cmd/image-service/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
