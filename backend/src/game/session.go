package game

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// SaveFormat versions the files written under the save directory. Bump it
// whenever a field's meaning changes; readers must keep loading older formats
// (charter R15.2, R22.2).
const SaveFormat = 1

const (
	sessionsDirName  = "sessions"
	sessionInfoName  = "session.json"
	commandLogName   = "commands.jsonl"
	objectiveLogName = "objectives.jsonl"
)

// SessionInfo identifies what a command log was recorded against, so a replay
// can refuse a different scenario or firmware and warn on a newer revision
// (scenario format §9).
type SessionInfo struct {
	Format   int    `json:"format"`
	Scenario string `json:"scenario"`
	Revision int    `json:"revision"`
	AppCRC32 string `json:"app_crc32"`
	Started  string `json:"started"`
}

// CommandRecord is one line of commands.jsonl (scenario format §8, charter
// R22.1). Wall is for people reading the log and must never schedule replay.
type CommandRecord struct {
	Seq     int     `json:"seq"`
	TMs     *uint64 `json:"t_ms"`
	Raw     string  `json:"raw"`
	Verb    string  `json:"verb"`
	Charged *string `json:"charged"`
	Result  *string `json:"result"`
	Wall    string  `json:"wall"`
}

// ObjectiveRecord is one line of objectives.jsonl: an objective changing
// status. It is derived from the command log, which replay restores state
// from; it records when things happened so they can be read without a replay.
type ObjectiveRecord struct {
	Frame     int             `json:"frame"`
	UptimeS   uint32          `json:"uptime_s"`
	Objective string          `json:"objective"`
	From      ObjectiveStatus `json:"from"`
	State     ObjectiveStatus `json:"state"`
	Wall      string          `json:"wall"`
}

// Session appends a game's command log and objective changes to the save
// directory.
type Session struct {
	Dir          string
	log          *os.File
	objectiveLog *os.File
	seq          int
}

// now is the wall clock, swapped out by tests.
var now = time.Now

// NewSession creates a session directory for scenario under saveDir.
func NewSession(saveDir string, scenario *Scenario) (*Session, error) {
	started := now().UTC()

	sessionsDir := filepath.Join(saveDir, sessionsDirName)
	if err := os.MkdirAll(sessionsDir, 0o755); err != nil {
		return nil, fmt.Errorf("creating save directory: %w", err)
	}

	dir := filepath.Join(sessionsDir,
		fmt.Sprintf("%s-%s", started.Format("20060102T150405.000Z"), scenario.Manifest.ID))
	if err := os.Mkdir(dir, 0o755); err != nil {
		return nil, fmt.Errorf("creating session directory: %w", err)
	}

	info := SessionInfo{
		Format:   SaveFormat,
		Scenario: scenario.Manifest.ID,
		Revision: scenario.Manifest.Revision,
		AppCRC32: scenario.Manifest.Firmware.AppCRC32,
		Started:  started.Format(time.RFC3339Nano),
	}
	if err := writeFileSynced(filepath.Join(dir, sessionInfoName), info); err != nil {
		return nil, err
	}

	log, err := createLog(filepath.Join(dir, commandLogName))
	if err != nil {
		return nil, fmt.Errorf("creating command log: %w", err)
	}

	objectiveLog, err := createLog(filepath.Join(dir, objectiveLogName))
	if err != nil {
		log.Close()
		return nil, fmt.Errorf("creating objective log: %w", err)
	}

	return &Session{Dir: dir, log: log, objectiveLog: objectiveLog}, nil
}

func createLog(path string) (*os.File, error) {
	return os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY|os.O_APPEND, 0o644)
}

// Append records a command and its reply, syncing so it survives a crash.
// An empty Result records a command the probe never answered.
func (s *Session) Append(command CommandLogEntry) error {
	if s == nil {
		return nil
	}

	s.seq++
	record := CommandRecord{
		Seq:     s.seq,
		TMs:     command.TMs,
		Raw:     command.Raw,
		Verb:    command.Verb,
		Charged: nullable(command.Charged),
		Result:  nullable(command.Result),
		Wall:    now().UTC().Format(time.RFC3339Nano),
	}

	return appendSynced(s.log, record)
}

// AppendObjective records an objective changing status, syncing so a
// completion reaches the save volume at once (charter R3.3).
func (s *Session) AppendObjective(transition ObjectiveTransition) error {
	if s == nil {
		return nil
	}

	return appendSynced(s.objectiveLog, ObjectiveRecord{
		Frame:     transition.Frame,
		UptimeS:   transition.UptimeS,
		Objective: transition.Objective,
		From:      transition.From,
		State:     transition.To,
		Wall:      now().UTC().Format(time.RFC3339Nano),
	})
}

func (s *Session) Close() error {
	if s == nil {
		return nil
	}
	return errors.Join(s.log.Close(), s.objectiveLog.Close())
}

func appendSynced(f *os.File, record any) error {
	line, err := json.Marshal(record)
	if err != nil {
		return err
	}
	if _, err := f.Write(append(line, '\n')); err != nil {
		return fmt.Errorf("appending to %s: %w", filepath.Base(f.Name()), err)
	}
	return f.Sync()
}

func nullable(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// writeFileSynced writes v as JSON to path atomically: readers see either the
// old file or the complete new one.
func writeFileSynced(path string, v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}

	tmp, err := os.CreateTemp(filepath.Dir(path), filepath.Base(path)+".*.tmp")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())

	if _, err := tmp.Write(append(data, '\n')); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}
