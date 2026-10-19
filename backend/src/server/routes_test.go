package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sojourn/app"
	"sojourn/emulator"
	"sojourn/game"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

func TestMain(m *testing.M) {
	app.InitLoggers()
	os.Exit(m.Run())
}

func intPtr(v int) *int { return &v }

// newTestServer returns a server in the level-select phase with the given
// scenarios available.
func newTestServer(t *testing.T, scenarios ...*game.Scenario) *server {
	t.Helper()
	s := newServer()
	if err := s.gs.LoadScenarios(t.TempDir()); err != nil {
		t.Fatalf("LoadScenarios: %v", err)
	}
	for _, sc := range scenarios {
		s.gs.AvailableScenarios[sc.Manifest.ID] = sc
	}
	return s
}

func testScenario() *game.Scenario {
	sc := &game.Scenario{
		Dir: "/nonexistent",
		Manifest: game.Manifest{
			ID:         "test.scenario",
			Title:      "Test Scenario",
			Revision:   3,
			Author:     "Tester",
			Summary:    "A scenario for tests",
			Difficulty: "easy",
		},
		Objectives: []game.Objective{
			{ID: "first", Title: "First", Brief: "Do the first thing", Success: game.Predicate{Op: "always"}},
			{ID: "second", Title: "Second", Brief: "Then the second", Requires: []string{"first"}, Success: game.Predicate{Op: "always"}},
		},
	}
	sc.Manifest.Link.Budget = game.BudgetConfig{Writes: intPtr(5), Reads: intPtr(10)}
	return sc
}

func TestScenariosMethodNotAllowed(t *testing.T) {
	s := newTestServer(t)
	rec := httptest.NewRecorder()
	s.scenarios(rec, httptest.NewRequest(http.MethodPost, "/api/scenarios", nil))

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusMethodNotAllowed)
	}
	if got := rec.Header().Get("Allow"); got != http.MethodGet {
		t.Errorf("Allow = %q, want %q", got, http.MethodGet)
	}
}

func TestScenariosEmpty(t *testing.T) {
	s := newTestServer(t)
	rec := httptest.NewRecorder()
	s.scenarios(rec, httptest.NewRequest(http.MethodGet, "/api/scenarios", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if got := strings.TrimSpace(rec.Body.String()); got != "[]" {
		t.Errorf("body = %q, want []", got)
	}
}

func TestScenariosResponse(t *testing.T) {
	noBudget := testScenario()
	noBudget.Manifest.ID = "test.nobudget"
	noBudget.Manifest.Link.Budget = game.BudgetConfig{}
	noBudget.Objectives = nil

	s := newTestServer(t, testScenario(), noBudget)
	rec := httptest.NewRecorder()
	s.scenarios(rec, httptest.NewRequest(http.MethodGet, "/api/scenarios", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	var resp scenariosResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(resp) != 2 {
		t.Fatalf("len = %d, want 2", len(resp))
	}

	byID := map[string]scenarioResponse{}
	for _, sr := range resp {
		byID[sr.ID] = sr
	}

	got := byID["test.scenario"]
	if got.Name != "Test Scenario" || got.Revision != 3 || got.Author != "Tester" ||
		got.Description != "A scenario for tests" || got.Difficulty != "easy" {
		t.Errorf("unexpected scenario fields: %+v", got)
	}
	if got.Constraints["writes"] != 5 || got.Constraints["reads"] != 10 || len(got.Constraints) != 2 {
		t.Errorf("constraints = %v", got.Constraints)
	}
	wantObjectives := []objectiveResponse{
		{ID: "first", Name: "First", Description: "Do the first thing", Status: game.ObjectiveStatusActive},
		{ID: "second", Name: "Second", Description: "Then the second", Status: game.ObjectiveStatusLocked},
	}
	if len(got.Objectives) != len(wantObjectives) {
		t.Fatalf("objectives = %+v", got.Objectives)
	}
	for i, want := range wantObjectives {
		if got.Objectives[i] != want {
			t.Errorf("objective %d = %+v, want %+v", i, got.Objectives[i], want)
		}
	}

	nb := byID["test.nobudget"]
	if len(nb.Constraints) != 0 {
		t.Errorf("constraints = %v, want empty", nb.Constraints)
	}
	if nb.Objectives == nil || len(nb.Objectives) != 0 {
		t.Errorf("objectives = %#v, want empty non-nil", nb.Objectives)
	}
}

func TestScenariosFromScenariosDir(t *testing.T) {
	dir := filepath.Join("..", "..", "..", "deps", "sojourn", "scenarios")
	if _, err := os.Stat(dir); err != nil {
		t.Skipf("scenarios dir not available: %v", err)
	}

	s := newServer()
	if err := s.gs.LoadScenarios(dir); err != nil {
		t.Skipf("unable to load scenarios: %v", err)
	}

	rec := httptest.NewRecorder()
	s.scenarios(rec, httptest.NewRequest(http.MethodGet, "/api/scenarios", nil))

	var resp scenariosResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(resp) != len(s.gs.AvailableScenarios) {
		t.Errorf("len = %d, want %d", len(resp), len(s.gs.AvailableScenarios))
	}
	for _, sr := range resp {
		if sr.ID == "" || sr.Name == "" {
			t.Errorf("scenario missing id/name: %+v", sr)
		}
	}
}

type failingWriter struct {
	header http.Header
	code   int
}

func (w *failingWriter) Header() http.Header        { return w.header }
func (w *failingWriter) Write([]byte) (int, error)  { return 0, os.ErrClosed }
func (w *failingWriter) WriteHeader(statusCode int) { w.code = statusCode }

func TestScenariosEncodeError(t *testing.T) {
	s := newTestServer(t, testScenario())
	w := &failingWriter{header: http.Header{}}
	s.scenarios(w, httptest.NewRequest(http.MethodGet, "/api/scenarios", nil))

	if w.code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", w.code, http.StatusInternalServerError)
	}
}

func TestSelectScenarioMissingParam(t *testing.T) {
	s := newTestServer(t, testScenario())
	rec := httptest.NewRecorder()
	s.selectScenario(rec, httptest.NewRequest(http.MethodGet, "/api/select-scenario", nil))

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
	if !strings.Contains(rec.Body.String(), "scenario-id") {
		t.Errorf("body = %q", rec.Body.String())
	}
	if s.gs.Phase() != game.PhaseLevelSelect {
		t.Errorf("phase = %v, want PhaseLevelSelect", s.gs.Phase())
	}
}

func TestSelectScenarioUnknownID(t *testing.T) {
	s := newTestServer(t, testScenario())
	rec := httptest.NewRecorder()
	s.selectScenario(rec, httptest.NewRequest(http.MethodGet, "/api/select-scenario?scenario-id=nope", nil))

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
	if !strings.Contains(rec.Body.String(), "nope") {
		t.Errorf("body = %q", rec.Body.String())
	}
	if s.gs.Phase() != game.PhaseLevelSelect {
		t.Errorf("phase = %v, want PhaseLevelSelect", s.gs.Phase())
	}
}

func TestSelectScenarioWrongPhase(t *testing.T) {
	s := newServer()
	s.gs.AvailableScenarios["test.scenario"] = testScenario()
	rec := httptest.NewRecorder()
	s.selectScenario(rec, httptest.NewRequest(http.MethodGet, "/api/select-scenario?scenario-id=test.scenario", nil))

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestSelectScenarioNotWebsocket(t *testing.T) {
	s := newTestServer(t, testScenario())
	rec := httptest.NewRecorder()
	s.selectScenario(rec, httptest.NewRequest(http.MethodGet, "/api/select-scenario?scenario-id=test.scenario", nil))

	if rec.Code < 400 {
		t.Fatalf("status = %d, want an error status", rec.Code)
	}
	if s.wsConn != nil {
		t.Error("wsConn set after failed upgrade")
	}
	if s.downlinkChan != nil || s.uplinkChan != nil || s.gameRunningDone != nil {
		t.Error("channels created after failed upgrade")
	}
}

func TestUnimplementedHandlers(t *testing.T) {
	s := newTestServer(t)
	tests := []struct {
		name    string
		handler http.HandlerFunc
		method  string
	}{
		{"newScenario", s.newScenario, http.MethodPost},
		{"saves", s.saves, http.MethodGet},
		{"loadSave", s.loadSave, http.MethodPut},
		{"save", s.save, http.MethodPut},
		{"results", s.results, http.MethodGet},
		{"uploadPatch", s.uploadPatch, http.MethodPost},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			tt.handler(rec, httptest.NewRequest(tt.method, "/", nil))
			if rec.Code != http.StatusOK {
				t.Errorf("%s: status = %d, want %d", tt.method, rec.Code, http.StatusOK)
			}

			rec = httptest.NewRecorder()
			tt.handler(rec, httptest.NewRequest(http.MethodDelete, "/", nil))
			if rec.Code != http.StatusMethodNotAllowed {
				t.Errorf("DELETE: status = %d, want %d", rec.Code, http.StatusMethodNotAllowed)
			}
			if got := rec.Header().Get("Allow"); got != tt.method {
				t.Errorf("Allow = %q, want %q", got, tt.method)
			}
		})
	}
}

func TestRoutes(t *testing.T) {
	buildDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(buildDir, "index.html"), []byte("hello frontend"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("BUILD_DIR", buildDir)

	s := newTestServer(t, testScenario())
	ts := httptest.NewServer(s.routes())
	defer ts.Close()

	tests := []struct {
		method string
		path   string
		want   int
	}{
		{http.MethodGet, "/", http.StatusOK},
		{http.MethodGet, "/missing.js", http.StatusNotFound},
		{http.MethodGet, "/api/scenarios", http.StatusOK},
		{http.MethodGet, "/api/select-scenario", http.StatusBadRequest},
		{http.MethodGet, "/api/new-scenario", http.StatusMethodNotAllowed},
		{http.MethodPost, "/api/saves", http.StatusMethodNotAllowed},
		{http.MethodGet, "/api/load-save", http.StatusMethodNotAllowed},
		{http.MethodGet, "/api/save", http.StatusMethodNotAllowed},
		{http.MethodPost, "/api/results", http.StatusMethodNotAllowed},
		{http.MethodGet, "/api/upload-patch", http.StatusMethodNotAllowed},
	}

	for _, tt := range tests {
		req, err := http.NewRequest(tt.method, ts.URL+tt.path, nil)
		if err != nil {
			t.Fatal(err)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("%s %s: %v", tt.method, tt.path, err)
		}
		resp.Body.Close()
		if resp.StatusCode != tt.want {
			t.Errorf("%s %s: status = %d, want %d", tt.method, tt.path, resp.StatusCode, tt.want)
		}
	}
}

func TestRoutesDefaultBuildDir(t *testing.T) {
	t.Setenv("BUILD_DIR", "")
	s := newTestServer(t)
	ts := httptest.NewServer(s.routes())
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/scenarios")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
}

func TestServeListenError(t *testing.T) {
	t.Setenv("SCENARIOS_DIR", t.TempDir())
	t.Setenv("PORT", "not-a-port")

	done := make(chan struct{})
	go func() {
		Serve()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Serve did not return on listen error")
	}
}

// wsPair upgrades a connection on an httptest server, hands the server-side
// conn to onConn, and returns the client-side conn.
func wsPair(t *testing.T, onConn func(*websocket.Conn)) *websocket.Conn {
	t.Helper()
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Errorf("upgrade: %v", err)
			return
		}
		onConn(conn)
	}))
	t.Cleanup(ts.Close)

	client, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(ts.URL, "http"), nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	t.Cleanup(func() { client.Close() })
	return client
}

func TestDownlinkPump(t *testing.T) {
	s := newTestServer(t)
	s.downlinkChan = make(chan game.DownlinkMessage)
	pumpDone := make(chan struct{})

	client := wsPair(t, func(conn *websocket.Conn) {
		s.wsConn = conn
		go func() {
			s.downlinkPump()
			close(pumpDone)
		}()
	})
	client.SetReadDeadline(time.Now().Add(5 * time.Second))

	frame := &emulator.TelemetryFrame{CRCOK: true, Frame: 7, Mode: "SAFE"}
	s.downlinkChan <- &game.TLMDownlinkMessage{TLM: "TLM abc", Telemetry: frame}
	var tlm telemetryDownlink
	if err := client.ReadJSON(&tlm); err != nil {
		t.Fatalf("read telemetry: %v", err)
	}
	if tlm.Type != "telemetry" || tlm.TLM != "TLM abc" || tlm.Telemetry == nil ||
		tlm.Telemetry.Frame != 7 || tlm.Telemetry.Mode != "SAFE" || !tlm.Telemetry.CRCOK {
		t.Errorf("telemetry = %+v", tlm)
	}

	s.downlinkChan <- &game.CommandResponseDownlinkMessage{Response: "ACK PING"}
	var cr commandResponseDownlink
	if err := client.ReadJSON(&cr); err != nil {
		t.Fatalf("read command response: %v", err)
	}
	if cr.Type != "commandResponse" || cr.Response != "ACK PING" {
		t.Errorf("command response = %+v", cr)
	}

	s.downlinkChan <- &game.StateDownlinkMessage{
		Objectives:  []game.ObjectiveStateDownlinkMessage{{ID: "first", Status: game.ObjectiveStatusComplete}},
		Constraints: map[string]int{"writes": 2},
	}
	var st stateDownlink
	if err := client.ReadJSON(&st); err != nil {
		t.Fatalf("read state: %v", err)
	}
	if st.Type != "state" || len(st.Objectives) != 1 || st.Objectives[0].ID != "first" ||
		st.Objectives[0].Status != game.ObjectiveStatusComplete || st.Constraints["writes"] != 2 {
		t.Errorf("state = %+v", st)
	}

	close(s.downlinkChan)
	select {
	case <-pumpDone:
	case <-time.After(5 * time.Second):
		t.Fatal("downlink pump did not exit after channel close")
	}
}

func TestDownlinkPumpWriteError(t *testing.T) {
	s := newTestServer(t)
	s.downlinkChan = make(chan game.DownlinkMessage)
	pumpDone := make(chan struct{})

	wsPair(t, func(conn *websocket.Conn) {
		s.wsConn = conn
		conn.Close()
		go func() {
			s.downlinkPump()
			close(pumpDone)
		}()
	})

	s.downlinkChan <- &game.CommandResponseDownlinkMessage{Response: "lost"}
	select {
	case <-pumpDone:
	case <-time.After(5 * time.Second):
		t.Fatal("downlink pump did not exit after write error")
	}
}

func TestUplinkPump(t *testing.T) {
	codes := []int{
		websocket.CloseNormalClosure,
		websocket.CloseGoingAway,
	}

	for _, code := range codes {
		t.Run(strconv.Itoa(code), func(t *testing.T) {
			s := newTestServer(t)
			s.uplinkChan = make(chan []byte)
			s.gameRunningDone = make(chan struct{})
			pumpDone := make(chan struct{})

			client := wsPair(t, func(conn *websocket.Conn) {
				s.wsConn = conn
				go func() {
					s.uplinkPump()
					close(pumpDone)
				}()
			})

			if err := client.WriteMessage(websocket.TextMessage, []byte("PING")); err != nil {
				t.Fatalf("write: %v", err)
			}
			select {
			case msg := <-s.uplinkChan:
				if string(msg) != "PING" {
					t.Errorf("uplink = %q, want PING", msg)
				}
			case <-time.After(5 * time.Second):
				t.Fatal("no uplink message")
			}

			closeMsg := websocket.FormatCloseMessage(code, "bye")
			if err := client.WriteControl(websocket.CloseMessage, closeMsg, time.Now().Add(time.Second)); err != nil {
				t.Fatalf("write close: %v", err)
			}

			select {
			case <-pumpDone:
			case <-time.After(5 * time.Second):
				t.Fatal("uplink pump did not exit on close")
			}
			if _, ok := <-s.uplinkChan; ok {
				t.Error("uplinkChan not closed")
			}
			if _, ok := <-s.gameRunningDone; ok {
				t.Error("gameRunningDone not closed")
			}
		})
	}
}

func TestUplinkPumpAbnormalClosure(t *testing.T) {
	s := newTestServer(t)
	s.uplinkChan = make(chan []byte)
	s.gameRunningDone = make(chan struct{})
	pumpDone := make(chan struct{})

	client := wsPair(t, func(conn *websocket.Conn) {
		s.wsConn = conn
		go func() {
			s.uplinkPump()
			close(pumpDone)
		}()
	})
	client.NetConn().Close()

	select {
	case <-pumpDone:
	case <-time.After(5 * time.Second):
		t.Fatal("uplink pump did not exit on abnormal closure")
	}
	if _, ok := <-s.gameRunningDone; ok {
		t.Error("gameRunningDone not closed")
	}
}
