package main

import (
	"fmt"
	"os"
	"sojourn/app"
	"sojourn/server"
)

func main() {
	programName := os.Args[0]

	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "Usage: %s firmware\n", programName)
		os.Exit(1)
	}
	app.InitLoggers()

	server.Serve()
}
