package server

import (
	"fmt"
	"net/http"
	"os"
	"sojourn/app"
	"sojourn/emulator"
	"strings"
)

func Serve() {
	downlinkChan := make(chan []byte)

	go downlinkMessagePrinter(downlinkChan)

	go emulator.LoadAndStartFirmware(os.Args[1], downlinkChan)

	mux := routes()
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	addr := fmt.Sprintf("0.0.0.0:%s", port)

	app.ServerLogger.Printf("Listening and serving on %s... ", addr)

	err := http.ListenAndServe(addr, mux)

	if err != nil {
		app.ErrorLogger.Printf("ListenAndServe error: %+v", err)
	}
}

func routes() *http.ServeMux {
	mux := http.NewServeMux()

	buildDir := os.Getenv("BUILD_DIR")
	if buildDir == "" {
		buildDir = "../../frontend/build"
	}

	fileServer := http.FileServer(http.Dir(buildDir))
	mux.Handle("/", fileServer)

	return mux
}

func downlinkMessagePrinter(downlinkChan chan []byte) {
	for message := range downlinkChan {
		line := strings.TrimSpace(string(message))

		if strings.HasPrefix(line, "TLM ") {
			frame, err := emulator.DecodeTelemetryFrame(line[4:])
			if err != nil {
				app.ErrorLogger.Printf("Failed to decode telemetry frame: %v", err)
				continue
			}

			emulator.PrintTelemetryFrame(frame)
		}
	}
}
