package main

import (
	"sojourn/app"
	"sojourn/server"
)

func main() {
	app.InitLoggers()
	server.Serve()
}
