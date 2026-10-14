package server

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"sojourn/app"
	"sojourn/emulator"
	"sojourn/game"
	"strconv"
	"strings"

	"github.com/gorilla/websocket"
)

type server struct {
	gs     game.State
	grader *game.Grader

	wsConn *websocket.Conn

	downlinkChan chan []byte
	uplinkChan   chan []byte

	firmwareDone chan struct{}
}

func newServer() *server {
	return &server{gs: game.InitState()}
}

func Serve() {
	server := newServer()

	scenariosDir := os.Getenv("SCENARIOS_DIR")
	if scenariosDir == "" {
		scenariosDir = "../../deps/sojourn/scenarios/"
	}

	server.gs.LoadScenarios(scenariosDir)

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

	response := scenariosResponse{}

	for _, scenario := range s.gs.AvailableScenarios {
		objectives := []objectiveResponse{}
		for _, objective := range scenario.Objectives {
			objectives = append(objectives, objectiveResponse{
				ID:          objective.ID,
				Name:        objective.Title,
				Description: objective.Brief,
				Status:      scenario.ObjectiveStates[objective.ID].Status,
			})
		}

		constraints := map[string]int{}

		writes := scenario.Manifest.Link.Budget.Writes
		reads := scenario.Manifest.Link.Budget.Reads

		if writes != nil {
			constraints["writes"] = *writes
		}

		if reads != nil {
			constraints["reads"] = *reads
		}

		sr := scenarioResponse{
			ID:          scenario.Manifest.ID,
			Name:        scenario.Manifest.Title,
			Revision:    scenario.Manifest.Revision,
			Author:      scenario.Manifest.Author,
			Description: scenario.Manifest.Summary,
			Difficulty:  scenario.Manifest.Difficulty,

			Objectives: objectives,

			Constraints: constraints,
		}

		response = append(response, sr)
	}

	err := json.NewEncoder(w).Encode(response)
	if err != nil {
		http.Error(w, "Failed to encode JSON", http.StatusInternalServerError)
		return
	}
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

	s.wsConn = conn
	s.downlinkChan = make(chan []byte)
	s.uplinkChan = make(chan []byte)

	s.firmwareDone = make(chan struct{})

	s.grader = game.NewGrader(&s.gs, net.JoinHostPort(emulator.GDBTCPAddr, strconv.Itoa(emulator.GDBTCPPort)))

	go emulator.LoadAndStartFirmware(os.Args[1], s.downlinkChan, s.uplinkChan, s.firmwareDone)

	go s.downlinkPump()
	go s.uplinkPump()
}

func (s *server) downlinkPump() {
	app.ServerLogger.Println("Starting downlink pump")

	defer s.grader.Close()

	for downlinkMessage := range s.downlinkChan {
		line := strings.TrimSpace(string(downlinkMessage))

		var err error

		if strings.HasPrefix(line, "TLM ") {
			frame, err := emulator.DecodeTelemetryFrame(line[4:])
			if err != nil {
				app.ErrorLogger.Printf("Failed to decode telemetry frame: %v", err)
				continue
			}

			emulator.PrintTelemetryFrame(frame)

			if err := s.grader.Grade(frame); err != nil {
				app.ErrorLogger.Printf("Grading: %v", err)
			}

			// TODO: send state downlink

			t := telemetryDownlink{
				Type:      "telemetry",
				TLM:       line,
				Telemetry: frame,
			}

			err = s.wsConn.WriteJSON(t)
		} else {
			app.ServerLogger.Printf("Command Response: `%s`\n", line)

			r := commandResponseDownlink{
				Type:     "commandResponse",
				Response: line,
			}

			err = s.wsConn.WriteJSON(r)
		}

		if err != nil {
			app.ServerLogger.Printf("Error writing JSON, probably WS connection closed: %+v\n", err)
			break
		}
	}

	app.ServerLogger.Println("Exiting downlink pump")
}

func (s *server) uplinkPump() {
	for {
		_, uplinkMessage, err := s.wsConn.ReadMessage()
		if err != nil {
			if websocket.IsCloseError(err,
				websocket.CloseNormalClosure,
				websocket.CloseGoingAway,
				websocket.CloseAbnormalClosure,
			) {
				app.ServerLogger.Printf("Client closed websocket: %+v", err)
				break
			}

			app.ErrorLogger.Printf("Error reading WS message: %+v", err)
			continue
		}

		app.ServerLogger.Printf("Uplink message received: `%s`\n", uplinkMessage)

		s.uplinkChan <- uplinkMessage
	}

	close(s.uplinkChan)

	// Tell emulator thread to kill firmware
	close(s.firmwareDone)

	app.ServerLogger.Println("Exiting uplink pump")
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
