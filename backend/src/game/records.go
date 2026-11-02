package game

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"time"

	"sojourn/app"
)

type ScenarioRecord struct {
	Session    string          `json:"session"`
	Revision   int             `json:"revision"`
	ElapsedMS  int64           `json:"elapsed_ms"`
	Uplinks    int             `json:"uplinks"`
	Objectives []ObjectiveTime `json:"objectives"`
}

type ObjectiveTime struct {
	ID        string `json:"id"`
	ElapsedMS int64  `json:"elapsed_ms"`
	Uplinks   int    `json:"uplinks"`
}

func BestRecords(saveDir string, scenarios map[string]*Scenario) (map[string]*ScenarioRecord, error) {
	records := map[string]*ScenarioRecord{}
	if saveDir == "" {
		return records, nil
	}

	dirs, err := os.ReadDir(filepath.Join(saveDir, sessionsDirName))
	if errors.Is(err, fs.ErrNotExist) {
		return records, nil
	}
	if err != nil {
		return nil, err
	}

	for _, dir := range dirs {
		if !dir.IsDir() {
			continue
		}

		record, scenarioID, err := sessionRecord(filepath.Join(saveDir, sessionsDirName, dir.Name()), scenarios)
		if err != nil {
			app.ErrorLogger.Printf("Skipping session %s: %v", dir.Name(), err)
			continue
		}
		if record != nil && faster(record, records[scenarioID]) {
			records[scenarioID] = record
		}
	}

	return records, nil
}

func faster(a, b *ScenarioRecord) bool {
	if b == nil {
		return true
	}
	if a.ElapsedMS != b.ElapsedMS {
		return a.ElapsedMS < b.ElapsedMS
	}
	if a.Uplinks != b.Uplinks {
		return a.Uplinks < b.Uplinks
	}
	return a.Session < b.Session
}

func sessionRecord(dir string, scenarios map[string]*Scenario) (*ScenarioRecord, string, error) {
	data, err := os.ReadFile(filepath.Join(dir, sessionInfoName))
	if err != nil {
		return nil, "", err
	}
	var info SessionInfo
	if err := json.Unmarshal(data, &info); err != nil {
		return nil, "", fmt.Errorf("reading %s: %w", sessionInfoName, err)
	}

	scenario, ok := scenarios[info.Scenario]
	if !ok {
		return nil, info.Scenario, nil
	}
	started, err := time.Parse(time.RFC3339Nano, info.Started)
	if err != nil {
		return nil, "", fmt.Errorf("reading start time: %w", err)
	}

	type transition struct {
		Objective string          `json:"objective"`
		State     ObjectiveStatus `json:"state"`
		Wall      string          `json:"wall"`
	}
	completed := map[string]time.Time{}
	err = readJSONLines(filepath.Join(dir, objectiveLogName), func(t transition) error {
		if t.State != ObjectiveStatusComplete {
			return nil
		}
		if _, ok := completed[t.Objective]; ok {
			return nil
		}
		wall, err := time.Parse(time.RFC3339Nano, t.Wall)
		if err != nil {
			return err
		}
		completed[t.Objective] = wall
		return nil
	})
	if err != nil {
		return nil, "", err
	}

	var objectives []ObjectiveTime
	for _, objective := range scenario.Objectives {
		wall, ok := completed[objective.ID]
		if !ok {
			return nil, info.Scenario, nil
		}
		objectives = append(objectives, ObjectiveTime{ID: objective.ID, ElapsedMS: wall.Sub(started).Milliseconds()})
	}

	var commandWalls []time.Time
	err = readJSONLines(filepath.Join(dir, commandLogName), func(c struct {
		Wall string `json:"wall"`
	}) error {
		wall, err := time.Parse(time.RFC3339Nano, c.Wall)
		if err != nil {
			return err
		}
		commandWalls = append(commandWalls, wall)
		return nil
	})
	if err != nil {
		return nil, "", err
	}

	// Split the uplinks between objectives in the order they completed.
	order := make([]int, len(objectives))
	for i := range order {
		order[i] = i
	}
	slices.SortStableFunc(order, func(a, b int) int {
		return completed[objectives[a].ID].Compare(completed[objectives[b].ID])
	})

	counted := 0
	for _, i := range order {
		wall := completed[objectives[i].ID]
		for counted < len(commandWalls) && !commandWalls[counted].After(wall) {
			objectives[i].Uplinks++
			counted++
		}
	}

	last := objectives[order[len(order)-1]]
	return &ScenarioRecord{
		Session:    filepath.Base(dir),
		Revision:   info.Revision,
		ElapsedMS:  last.ElapsedMS,
		Uplinks:    counted,
		Objectives: objectives,
	}, info.Scenario, nil
}

func readJSONLines[T any](path string, each func(T) error) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		var value T
		if err := json.Unmarshal(scanner.Bytes(), &value); err != nil {
			continue
		}
		if err := each(value); err != nil {
			return err
		}
	}
	return scanner.Err()
}
