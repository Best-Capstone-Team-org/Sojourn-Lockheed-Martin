package server

import (
	"sojourn/emulator"
	"sojourn/game"
)

type scenariosResponse []struct {
	Id          int    `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

type scenarioResponse struct {
	Name        string `json:"id"`
	Description string `json:"description"`

	Antennas []struct {
		Id   int    `json:"id"`
		Name string `json:"name"`
		// TODO:
	} `json:"antennas"`

	Objectives []struct {
		Name string `json:"name"`
		// TODO:
	} `json:"objectives"`

	Constraints map[string]int `json:"constraints"`
}

type selectScenarioResponse struct {
	Message string `json:"message"`
}

type newScenarioResponse struct {
	Message string `json:"message"`
}

type savesResponse []struct {
	Id         int    `json:"id"`
	Timestamp  string `json:"timestamp"`
	Objectives []struct {
		Name   string               `json:"Name"`
		Status game.ObjectiveStatus `json:"status"`
	} `json:"objectives"`
	Constraints map[string]int `json:"constraints"`
}

type loadSaveResponse struct {
	Message string `json:"message"`
}

type saveResponse struct {
	Message string `json:"message"`
}

type resultsResponse struct {
	// TODO:
}

type uploadPatch struct {
	Message string `json:"message"`
}

// Websocket responses

type telemetryDownlink struct {
	Type      string                   `json:"type"`
	TLM       string                   `json:"TLM"`
	Telemetry *emulator.TelemetryFrame `json:"telemetry"`
}

type stateDownlink struct {
	Type       string `json:"type"`
	Objectives []struct {
		Name   string               `json:"name"`
		Status game.ObjectiveStatus `json:"status"`
	} `json:"objectives"`
	Constraints map[string]int `json:"constraints"`
}
