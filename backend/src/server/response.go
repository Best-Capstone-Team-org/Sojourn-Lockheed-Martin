package server

import (
	"sojourn/emulator"
	"sojourn/game"
)

type scenariosResponse []scenarioResponse

type objectiveResponse struct {
	ID          string               `json:"id"`
	Name        string               `json:"name"`
	Description string               `json:"description"`
	Status      game.ObjectiveStatus `json:"status"`
}

type scenarioResponse struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Revision    int    `json:"revision"`
	Author      string `json:"author"`
	Description string `json:"description"`
	Difficulty  string `json:"difficulty"`

	Objectives []objectiveResponse `json:"objectives"`

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

type commandResponseDownlink struct {
	Type     string `json:"type"`
	Response string `json:"response"`
}

type stateDownlink struct {
	Type       string `json:"type"`
	Objectives []struct {
		Name   string               `json:"name"`
		Status game.ObjectiveStatus `json:"status"`
	} `json:"objectives"`
	Constraints map[string]int `json:"constraints"`
}
