package main

import (
	"fmt"
	"os"
)

func read(filename string) {
	data, err := os.ReadFile(filename)
	if err != nil {
		Errorf("Read file %s failed, %v", filename, err)
		os.Exit(1)
	}

	_, _ = fmt.Fprintf(os.Stdout, "%s", data)
}
