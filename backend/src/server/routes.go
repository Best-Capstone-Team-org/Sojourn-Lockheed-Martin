package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"sojourn/app"
	"sojourn/game"
	"strconv"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

type server struct {
	gs game.State

	wsConn *websocket.Conn

	downlinkChan chan game.DownlinkMessage
	uplinkChan   chan []byte

	// uplinkMu guards closing uplinkChan against uploads queueing commands
	// on it from another request; uplinkOpen is false once it is closed.
	uplinkMu   sync.Mutex
	uplinkOpen bool

	gameRunningDone chan struct{}
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

	scenariosUserDir := os.Getenv("SCENARIOS_USER_DIR")

	err := server.gs.LoadScenarios([]game.ScenariosDir{
		{
			Dir:      scenariosDir,
			Required: true,
		},
		{
			Dir:      scenariosUserDir,
			Required: false,
		},
	})
	if err != nil {
		app.ErrorLogger.Fatalf("Unable to load scenarios: %v", err)
	}

	app.ServerLogger.Printf("Loaded %d scenarios from %s", len(server.gs.AvailableScenarios), scenariosDir)

	server.gs.SaveDir = os.Getenv("SAVE_DIR")
	if server.gs.SaveDir == "" {
		app.ErrorLogger.Println("SAVE_DIR is not set: sessions will not be saved")
	} else {
		app.ServerLogger.Printf("Saving sessions to %s", server.gs.SaveDir)
	}

	mux := server.routes()
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	addr := fmt.Sprintf("0.0.0.0:%s", port)

	app.ServerLogger.Printf("Listening and serving on %s... ", addr)

	err = http.ListenAndServe(addr, mux)

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
	mux.HandleFunc(apiPrefix+"/download-binary", s.downloadBinary)
	mux.HandleFunc(apiPrefix+"/download-golden-binary", s.downloadGoldenBinary)
	mux.HandleFunc(apiPrefix+"/upload-patch", s.uploadPatch)

	return mux
}

func (s *server) scenarios(w http.ResponseWriter, r *http.Request) {
	if !s.checkRequestMethod(r, http.MethodGet, w) {
		return
	}

	response := scenariosResponse{}

	records, err := game.BestRecords(s.gs.SaveDir, s.gs.AvailableScenarios)
	if err != nil {
		app.ErrorLogger.Printf("Reading scenario records: %v", err)
	}

	for _, scenario := range s.gs.AvailableScenarios {
		objectives := []objectiveResponse{}
		for _, objective := range scenario.Objectives {
			objectives = append(objectives, objectiveResponse{
				ID:          objective.ID,
				Name:        objective.Title,
				Description: objective.Brief,
				Status:      s.gs.ObjectiveStatus(scenario, objective),
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

			UplinkDelayS: scenario.Manifest.Link.UplinkDelayS,

			Record: records[scenario.Manifest.ID],
		}

		response = append(response, sr)
	}

	err = json.NewEncoder(w).Encode(response)
	if err != nil {
		http.Error(w, "Failed to encode JSON", http.StatusInternalServerError)
		return
	}
}

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

func (s *server) selectScenario(w http.ResponseWriter, r *http.Request) {
	queryParams := r.URL.Query()
	if !queryParams.Has("scenario-id") {
		http.Error(w, "scenario-id query parameter not supplied", http.StatusBadRequest)
		return
	}

	scenarioId := queryParams.Get("scenario-id")

	err := s.gs.SelectScenario(scenarioId)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		app.ErrorLogger.Printf("Error upgrading to WS: %+v", err)
		s.deselectScenario()
		return
	}

	s.wsConn = conn
	s.downlinkChan = make(chan game.DownlinkMessage)
	s.uplinkChan = make(chan []byte)
	s.uplinkOpen = true

	s.gameRunningDone = make(chan struct{})

	err = s.gs.Start(s.downlinkChan, s.uplinkChan, s.gameRunningDone)
	if err != nil {
		app.ErrorLogger.Printf("Error starting game: %v", err)
		closeMessage := websocket.FormatCloseMessage(websocket.CloseInternalServerErr,
			fmt.Sprintf("Error starting game: %v", err))
		conn.WriteControl(websocket.CloseMessage, closeMessage, time.Now().Add(time.Second))
		conn.Close()
		s.deselectScenario()
		return
	}

	go s.downlinkPump()
	go s.uplinkPump()
}

func (s *server) deselectScenario() {
	if err := s.gs.DeselectScenario(); err != nil {
		app.ErrorLogger.Printf("Error deselecting scenario: %v", err)
	}
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

// The game calls behind the binary endpoints. Tests replace them, since the
// real ones need a running probe.
var (
	runningImage    = (*game.State).RunningImage
	goldenImage     = (*game.State).GoldenImage
	patchPokes      = (*game.State).PatchPokes
	remainingWrites = (*game.State).RemainingWrites
)

// maxPatchBytes is APP RAM's size: no patch can be larger.
const maxPatchBytes = 0x18000

func gameErrorStatus(err error) int {
	switch {
	case errors.Is(err, game.ErrNoGameInProgress), errors.Is(err, game.ErrPatchTooLarge):
		return http.StatusBadRequest
	case errors.Is(err, game.ErrProbeMemoryUnavailable):
		return http.StatusServiceUnavailable
	default:
		return http.StatusInternalServerError
	}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

// uploadPatch turns an uploaded application image into POKEs for whatever
// differs from the image running now, and sends them as if typed.
// ?dry-run=1 reports them without sending.
func (s *server) uploadPatch(w http.ResponseWriter, r *http.Request) {
	if !s.checkRequestMethod(r, http.MethodPost, w) {
		return
	}

	patched, err := readPatch(w, r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, uploadPatchResponse{Message: err.Error()})
		return
	}

	pokes, err := patchPokes(&s.gs, patched)
	if err != nil {
		app.ErrorLogger.Printf("Error building patch: %v", err)
		writeJSON(w, gameErrorStatus(err), uploadPatchResponse{Message: err.Error()})
		return
	}

	response := uploadPatchResponse{Pokes: pokes, Writes: len(pokes)}
	if response.Pokes == nil {
		response.Pokes = []string{}
	}

	// A patch is sent whole or not at all: half a patch can leave the probe
	// running code that is neither old nor new.
	if remaining, metered := remainingWrites(&s.gs); metered && len(pokes) > remaining {
		response.Message = fmt.Sprintf("patch needs %d writes but only %d remain", len(pokes), remaining)
		writeJSON(w, http.StatusConflict, response)
		return
	}

	if r.URL.Query().Get("dry-run") != "" {
		response.DryRun = true
		writeJSON(w, http.StatusOK, response)
		return
	}

	if !s.queueUplinks(pokes) {
		response.Message = game.ErrNoGameInProgress.Error()
		writeJSON(w, http.StatusBadRequest, response)
		return
	}

	app.ServerLogger.Printf("Queued %d POKEs from an uploaded patch", len(pokes))
	writeJSON(w, http.StatusOK, response)
}

// readPatch reads the image from a multipart "file" field, or else the raw
// request body.
func readPatch(w http.ResponseWriter, r *http.Request) ([]byte, error) {
	r.Body = http.MaxBytesReader(w, r.Body, maxPatchBytes+64*1024)

	if mediaType, _, _ := mime.ParseMediaType(r.Header.Get("Content-Type")); mediaType == "multipart/form-data" {
		file, _, err := r.FormFile("file")
		if err != nil {
			return nil, fmt.Errorf("reading uploaded file: %w", err)
		}
		defer file.Close()
		return io.ReadAll(io.LimitReader(file, maxPatchBytes+1))
	}

	return io.ReadAll(io.LimitReader(r.Body, maxPatchBytes+1))
}

// queueUplinks sends commands down the uplink exactly as if the player had
// typed them, reporting false if the game's uplink is already closed.
func (s *server) queueUplinks(lines []string) bool {
	s.uplinkMu.Lock()
	defer s.uplinkMu.Unlock()

	if !s.uplinkOpen {
		return false
	}
	for _, line := range lines {
		s.uplinkChan <- []byte(line)
	}
	return true
}

func (s *server) downloadBinary(w http.ResponseWriter, r *http.Request) {
	s.serveImage(w, r, runningImage, "probe.bin")
}

func (s *server) downloadGoldenBinary(w http.ResponseWriter, r *http.Request) {
	s.serveImage(w, r, goldenImage, "probe_golden.bin")
}

func (s *server) serveImage(w http.ResponseWriter, r *http.Request,
	image func(*game.State) ([]byte, error), filename string) {
	if !s.checkRequestMethod(r, http.MethodGet, w) {
		return
	}

	data, err := image(&s.gs)
	if err != nil {
		app.ErrorLogger.Printf("Error reading app image: %v", err)
		writeJSON(w, gameErrorStatus(err), downloadBinaryResponse{Message: err.Error()})
		return
	}

	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))
	w.Header().Set("Content-Length", strconv.Itoa(len(data)))
	w.Write(data)
}

func (s *server) downlinkPump() {
	app.ServerLogger.Println("Starting downlink pump")

	for downlinkMessage := range s.downlinkChan {
		var err error

		switch downlinkMessage := downlinkMessage.(type) {
		case *game.TLMDownlinkMessage:
			err = s.wsConn.WriteJSON(telemetryDownlink{
				Type:      "telemetry",
				TLM:       downlinkMessage.TLM,
				Telemetry: downlinkMessage.Telemetry,
			})
		case *game.CommandResponseDownlinkMessage:
			err = s.wsConn.WriteJSON(commandResponseDownlink{
				Type:     "commandResponse",
				Response: downlinkMessage.Response,
			})
		case *game.StateDownlinkMessage:
			err = s.wsConn.WriteJSON(stateDownlink{
				Type:        "state",
				Objectives:  downlinkMessage.Objectives,
				Constraints: downlinkMessage.Constraints,
				Grading:     downlinkMessage.Grading,
			})
		case *game.CommandRefusedDownlinkMessage:
			err = s.wsConn.WriteJSON(commandRefusedDownlink{
				Type:        "commandRefused",
				Command:     downlinkMessage.Command,
				Reason:      downlinkMessage.Reason,
				Constraints: downlinkMessage.Constraints,
			})
		}

		if err != nil {
			app.ServerLogger.Printf("Error writing JSON, probably WS connection closed: %+v\n", err)
			s.wsConn.Close()
			break
		}
	}

	for range s.downlinkChan {
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
			} else {
				app.ErrorLogger.Printf("Error reading WS message: %+v", err)
			}
			break
		}

		app.ServerLogger.Printf("Uplink message received: `%s`\n", uplinkMessage)

		s.uplinkChan <- uplinkMessage
	}

	s.uplinkMu.Lock()
	s.uplinkOpen = false
	close(s.uplinkChan)
	s.uplinkMu.Unlock()

	// Tell emulator thread to kill firmware
	close(s.gameRunningDone)

	if err := s.gs.Stop(); err != nil {
		app.ErrorLogger.Println(fmt.Sprintf("Error stopping game: %s", err))
	}

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
