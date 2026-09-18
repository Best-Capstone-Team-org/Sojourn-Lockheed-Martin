export async function apiFetch(
	input: string | URL | Request,
	init?: RequestInit
): Promise<Response> {
	// TODO add prefix for backend
	return fetch(input, init);
}

// TODO create function for making websocket
// just pass functions to register
// need open, close, and message

export type ScenariosResponse = Scenario[];

export interface ScenarioResponse {
	name: string;
	description: string;
	antennas: {
		id: number;
		name: string;
	}[];
	objectives: {
		name: 'string';
		// TODO
	}[];
	constraints: Constraints;
}

export type SelectScenarioResponse = Message;

export type NewScenarioResponse = Message;

export type SavesResponse = Save[];

export type LoadSaveResponse = Message;

export type SaveResponse = Message;

export interface ResultsResponse {
	// TODO
}

export type UploadPatchResponse = Message;

export interface CommandUplink {
	command: {
		name: string;
		args: string[];
	};
	antenna: number;
}

export type CommandDownlink =
	| {
			type: 'telemetry';
			telemetry: Telemetry;
	  }
	| {
			type: 'state';
			objectives: Objective[];
			constraints: Constraints;
	  };

export interface Scenario {
	id: number;
	name: string;
	description: string;
	// TODO
}

export interface Save {
	id: number;
	timestamp: string; // ISO
	objectives: Objective[];
	constraints: Constraints;
}

export interface Message {
	message: string;
}

export interface Telemetry {
	// TODO
}

export interface Objective {
	name: string;
	status: 'complete' | 'failed' | 'active' | 'locked';
}

export interface Constraints {
	read: number;
	write: number;
	frame: number;
}
