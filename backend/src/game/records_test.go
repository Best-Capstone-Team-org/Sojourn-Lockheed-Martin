package game

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

var recordStart = time.Date(2026, 11, 2, 15, 0, 0, 0, time.UTC)

// step is one thing that happens in a recorded session, at seconds after its
// start: a command (raw set) or an objective changing status.
type step struct {
	at        float64
	raw       string
	objective string
	state     ObjectiveStatus
}

func command(at float64, raw string) step { return step{at: at, raw: raw} }

func completes(at float64, objective string) step {
	return step{at: at, objective: objective, state: ObjectiveStatusComplete}
}

func becomes(at float64, objective string, state ObjectiveStatus) step {
	return step{at: at, objective: objective, state: state}
}

// recordSession writes a session through the real Session API.
func recordSession(t *testing.T, saveDir string, scenario *Scenario, start time.Time, steps ...step) string {
	t.Helper()
	clock := start
	prev := now
	now = func() time.Time { return clock }
	defer func() { now = prev }()

	session, err := NewSession(saveDir, scenario)
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()

	for _, s := range steps {
		clock = start.Add(time.Duration(s.at * float64(time.Second)))
		if s.raw != "" {
			err = session.Append(parseCommand(s.raw))
		} else {
			err = session.AppendObjective(ObjectiveTransition{Objective: s.objective, From: ObjectiveStatusActive, To: s.state})
		}
		if err != nil {
			t.Fatal(err)
		}
	}
	return filepath.Base(session.Dir)
}

func recordScenario(id string, objectives ...string) *Scenario {
	s := &Scenario{Manifest: Manifest{ID: id, Revision: 2}}
	for _, objective := range objectives {
		s.Objectives = append(s.Objectives, Objective{ID: objective})
	}
	return s
}

func TestBestRecordsSplitsUplinksByObjective(t *testing.T) {
	saveDir := t.TempDir()
	// Declared b first, but a completes first.
	scenario := recordScenario("sojourn.test", "b", "a")

	session := recordSession(t, saveDir, scenario, recordStart,
		command(5, "PEEK 0x2001E200 16"),
		command(10, "PING"),
		completes(15, "a"),
		command(20, "POKE 0x2001E200 09"),
		completes(30, "b"),
		command(40, "PING"),
	)

	records, err := BestRecords(saveDir, map[string]*Scenario{"sojourn.test": scenario})
	if err != nil {
		t.Fatal(err)
	}
	want := &ScenarioRecord{
		Session:   session,
		Revision:  2,
		ElapsedMS: 30000,
		Uplinks:   3,
		Objectives: []ObjectiveTime{
			{ID: "b", ElapsedMS: 30000, Uplinks: 1},
			{ID: "a", ElapsedMS: 15000, Uplinks: 2},
		},
	}
	if got := records["sojourn.test"]; !reflect.DeepEqual(got, want) {
		t.Errorf("record = %+v, want %+v", got, want)
	}
}

func TestBestRecordsPicksFastest(t *testing.T) {
	saveDir := t.TempDir()
	scenario := recordScenario("sojourn.test", "only")
	scenarios := map[string]*Scenario{"sojourn.test": scenario}

	recordSession(t, saveDir, scenario, recordStart,
		command(1, "PING"), completes(60, "only"))
	fastest := recordSession(t, saveDir, scenario, recordStart.Add(time.Hour),
		command(1, "PING"), command(2, "PING"), completes(45, "only"))
	recordSession(t, saveDir, scenario, recordStart.Add(2*time.Hour),
		command(1, "PING"), command(2, "PING"), command(3, "PING"), completes(45, "only"))

	records, err := BestRecords(saveDir, scenarios)
	if err != nil {
		t.Fatal(err)
	}
	got := records["sojourn.test"]
	if got == nil || got.Session != fastest || got.ElapsedMS != 45000 || got.Uplinks != 2 {
		t.Errorf("record = %+v, want session %s: 45 s, and fewer uplinks on a tie", got, fastest)
	}
}

func TestBestRecordsNeedsEveryObjective(t *testing.T) {
	saveDir := t.TempDir()
	scenario := recordScenario("sojourn.test", "first", "second")

	recordSession(t, saveDir, scenario, recordStart, completes(10, "first"))
	recordSession(t, saveDir, scenario, recordStart.Add(time.Hour),
		completes(10, "first"), becomes(20, "second", ObjectiveStatusFailed))

	records, err := BestRecords(saveDir, map[string]*Scenario{"sojourn.test": scenario})
	if err != nil {
		t.Fatal(err)
	}
	if got, ok := records["sojourn.test"]; ok {
		t.Errorf("record = %+v from sessions that never completed every objective", got)
	}
}

func TestBestRecordsUsesFirstCompletion(t *testing.T) {
	saveDir := t.TempDir()
	scenario := recordScenario("sojourn.test", "held", "last")

	recordSession(t, saveDir, scenario, recordStart,
		completes(10, "held"),
		becomes(20, "held", ObjectiveStatusActive),
		completes(30, "held"),
		completes(40, "last"),
	)

	records, err := BestRecords(saveDir, map[string]*Scenario{"sojourn.test": scenario})
	if err != nil {
		t.Fatal(err)
	}
	if got := records["sojourn.test"]; got == nil || got.Objectives[0].ElapsedMS != 10000 {
		t.Errorf("record = %+v, want held first completed at 10 s", got)
	}
}

func TestBestRecordsSkipsWhatItCannotUse(t *testing.T) {
	saveDir := t.TempDir()
	mine := recordScenario("sojourn.mine", "only")
	other := recordScenario("sojourn.other", "only")
	uninstalled := recordScenario("sojourn.gone", "only")
	scenarios := map[string]*Scenario{"sojourn.mine": mine, "sojourn.other": other}

	session := recordSession(t, saveDir, mine, recordStart, command(1, "PING"), completes(20, "only"))
	recordSession(t, saveDir, other, recordStart.Add(time.Hour), completes(5, "only"))
	recordSession(t, saveDir, uninstalled, recordStart.Add(2*time.Hour), completes(1, "only"))

	// A crash mid-write leaves a torn last line, which is ignored.
	log, err := os.OpenFile(filepath.Join(saveDir, "sessions", session, "commands.jsonl"), os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	log.WriteString(`{"seq":2,"raw":"PI`)
	log.Close()

	// A session missing its metadata is skipped, not fatal.
	if err := os.MkdirAll(filepath.Join(saveDir, "sessions", "broken"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(saveDir, "sessions", "stray-file"), nil, 0o644); err != nil {
		t.Fatal(err)
	}

	records, err := BestRecords(saveDir, scenarios)
	if err != nil {
		t.Fatal(err)
	}
	if got := records["sojourn.mine"]; got == nil || got.Session != session || got.Uplinks != 1 || got.ElapsedMS != 20000 {
		t.Errorf("mine = %+v, want its own session: 20 s and 1 uplink", got)
	}
	if got := records["sojourn.other"]; got == nil || got.ElapsedMS != 5000 {
		t.Errorf("other = %+v, want 5 s", got)
	}
	if _, ok := records["sojourn.gone"]; ok {
		t.Error("record for a scenario that isn't installed")
	}
}

func TestBestRecordsWithoutSaves(t *testing.T) {
	scenarios := map[string]*Scenario{"sojourn.test": recordScenario("sojourn.test", "only")}
	for name, saveDir := range map[string]string{"unset": "", "empty": t.TempDir()} {
		records, err := BestRecords(saveDir, scenarios)
		if err != nil || len(records) != 0 {
			t.Errorf("%s save dir: %v, %v; want no records", name, records, err)
		}
	}

	notADir := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(notADir, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := BestRecords(notADir, scenarios); err == nil {
		t.Error("expected an error when the save directory can't be read")
	}
}
