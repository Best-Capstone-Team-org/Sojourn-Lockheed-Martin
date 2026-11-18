package main

import (
	"flag"
	"sojourn/app"
	"sojourn/server"
)

func main() {
	strict := flag.Bool("strict", false, "treat scenario validation warnings as failures")
	flag.Parse()

	app.InitLoggers()
	server.Serve(server.Options{StrictValidation: *strict})
}
