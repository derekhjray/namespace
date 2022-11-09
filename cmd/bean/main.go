package main

import (
	_ "github.com/opencontainers/runc/libcontainer/nsenter"
)

func main() {
	app.Execute()
}
