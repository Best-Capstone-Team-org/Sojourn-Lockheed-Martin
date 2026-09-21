package server

import (
	"fmt"
	"net/http"
	"os"
	"sojourn/app"
	"sojourn/emulator"
	"strings"

	"github.com/gorilla/websocket"
)

type server struct {
	// TODO: contain gamestate object from a different package probably

	conn *websocket.Conn

	downlinkChan chan []byte
}

func newServer() *server {
	return &server{}
}

func Serve() {
	server := newServer()

	mux := server.routes()
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

func (s *server) routes() *http.ServeMux {
	mux := http.NewServeMux()

	buildDir := os.Getenv("BUILD_DIR")
	if buildDir == "" {
		buildDir = "../../frontend/build"
	}

	fileServer := http.FileServer(http.Dir(buildDir))
	mux.Handle("/", fileServer)

	apiPrefix := "/api"

	mux.HandleFunc(apiPrefix+"/scenarios", s.scenarios)
	mux.HandleFunc(apiPrefix+"/scenario", s.scenario)
	mux.HandleFunc(apiPrefix+"/select-scenario", s.selectScenario)
	mux.HandleFunc(apiPrefix+"/new-scenario", s.newScenario)
	mux.HandleFunc(apiPrefix+"/saves", s.saves)
	mux.HandleFunc(apiPrefix+"/load-save", s.loadSave)
	mux.HandleFunc(apiPrefix+"/save", s.save)
	mux.HandleFunc(apiPrefix+"/results", s.results)
	mux.HandleFunc(apiPrefix+"/upload-patch", s.uploadPatch)
	mux.HandleFunc(apiPrefix+"/command-ws", s.commandWS)

	return mux
}

func (s *server) scenarios(w http.ResponseWriter, r *http.Request) {
	if !s.checkRequestMethod(r, http.MethodGet, w) {
		return
	}
	// TODO:
}

func (s *server) scenario(w http.ResponseWriter, r *http.Request) {
	if !s.checkRequestMethod(r, http.MethodGet, w) {
		return
	}
	// TODO:
}

func (s *server) selectScenario(w http.ResponseWriter, r *http.Request) {
	if !s.checkRequestMethod(r, http.MethodPut, w) {
		return
	}
	// TODO:
}

func (s *server) newScenario(w http.ResponseWriter, r *http.Request) {
	if !s.checkRequestMethod(r, http.MethodPost, w) {
		return
	}
	// TODO:
}

func (s *server) saves(w http.ResponseWriter, r *http.Request) {
	if !s.checkRequestMethod(r, http.MethodGet, w) {
		return
	}
	// TODO:
}

func (s *server) loadSave(w http.ResponseWriter, r *http.Request) {
	if !s.checkRequestMethod(r, http.MethodPut, w) {
		return
	}
	// TODO:
}

func (s *server) save(w http.ResponseWriter, r *http.Request) {
	if !s.checkRequestMethod(r, http.MethodPut, w) {
		return
	}
	// TODO:
}

func (s *server) results(w http.ResponseWriter, r *http.Request) {
	if !s.checkRequestMethod(r, http.MethodGet, w) {
		return
	}
	// TODO:
}

func (s *server) uploadPatch(w http.ResponseWriter, r *http.Request) {
	if !s.checkRequestMethod(r, http.MethodPost, w) {
		return
	}
	// TODO:
}

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

func (s *server) commandWS(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		app.ErrorLogger.Printf("Error upgrading to WS: %+v", err)
		s.clientError(w, http.StatusUpgradeRequired)
		return
	}

	s.conn = conn
	s.downlinkChan = make(chan []byte)

	go emulator.LoadAndStartFirmware(os.Args[1], s.downlinkChan)

	go s.downlinkPump()
	go s.uplinkPump()
}

func (s *server) downlinkPump() {
	app.ServerLogger.Println("Starting downlink pump")

	for downlinkMessage := range s.downlinkChan {
		line := strings.TrimSpace(string(downlinkMessage))

		if strings.HasPrefix(line, "TLM ") {
			frame, err := emulator.DecodeTelemetryFrame(line[4:])
			if err != nil {
				app.ErrorLogger.Printf("Failed to decode telemetry frame: %v", err)
				continue
			}

			emulator.PrintTelemetryFrame(frame)

			t := telemetryDownlink{
				Type:      "telemetry",
				TLM:       line,
				Telemetry: frame,
			}

			err = s.conn.WriteJSON(t)

			if err != nil {
				app.ServerLogger.Printf("Error wriring JSON, probably WS connection closed: %+v\n", err)
				break
			}
		}
	}

	app.ServerLogger.Println("Closing downlink pump")
}

func (s *server) uplinkPump() {
	for {
		_, message, err := s.conn.ReadMessage()
		if err != nil {
			app.ErrorLogger.Printf("Error reading WS message: %+v", err)
			continue
		}

		app.ServerLogger.Printf("Uplink message received: %s\n", message)
	}
}

func (s *server) checkRequestMethod(r *http.Request, method string, w http.ResponseWriter) bool {
	if r.Method != method {
		w.Header().Set("Allow", method)
		s.clientError(w, http.StatusMethodNotAllowed)
		return false
	}
	return true
}

func (s *server) clientError(w http.ResponseWriter, status int) {
	http.Error(w, http.StatusText(status), status)
}
