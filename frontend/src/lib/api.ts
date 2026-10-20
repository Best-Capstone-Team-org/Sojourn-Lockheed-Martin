export const API_ENDPOINT = "localhost:8080";
export const API_PREFIX = "api";

export async function apiFetch(
	endpoint: string,
	init?: RequestInit
): Promise<Response> {
	return fetch(`http://${API_ENDPOINT}/${API_PREFIX}/${endpoint}`, init);
}

export function apiWebSocket(endpoint: string) {
	return new WebSocket(`ws://${API_ENDPOINT}/${API_PREFIX}/${endpoint}`);
}

export type ScenariosResponse = ScenarioResponse[];

export interface ScenarioResponse {
	id: string;
	name: string;
	revision: number;
	author: string;
	description: string;
	difficulty: string;
	objectives: ObjectiveResponse[];
	constraints: Constraints;
}

export type ObjectiveStatus = "complete" | "failed" | "active" | "locked";

export interface ObjectiveResponse {
	id: string;
	name: string;
	description: string;
	status: ObjectiveStatus;
}

export type SelectScenarioResponse = Message;

export type NewScenarioResponse = Message;

export type SavesResponse = Save[];

export type LoadSaveResponse = Message;

export type SaveResponse = Message;

export interface ResultsResponse {
	// TODO
	todo: string;
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
			type: "telemetry";
			TLM: string;
			telemetry: Telemetry;
	  }
	| {
			type: "commandResponse";
			response: string;
	  }
	| {
			type: "state";
			objectives: Objective[];
			constraints: Constraints;
	  };

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
	crcOK: boolean;
	frame: number;
	uptime: number; // in seconds
	mode: string;
	reboots: number;
	lastFault: string;
	bus: number; // in MV
	load: number; // in MW
	sensors: Record<number, number>;

	camera?: Camera;
	hk?: HK;
	comms?: Comms;
	AUX?: number;

	unknown: Record<string, string>;
}

export interface Camera {
	frame: number;
	target: number;
	exposure: number; // in ms
	mean: number;
	sat: number;
	stars: number;
}

export interface HK {
	heaterOn: number;
	shed: number;
	prop: number; // propellant in mg
	mom: number; // momentum
	rec: number; // in percent
	auth: number;
}

export interface Comms {
	antenna: string;
	budget: number; // in B
	dropped: number;
	xStat: number;
	hgaDeploy: number; // in percent
}

export interface Objective {
	id: string;
	status: ObjectiveStatus;
	hint?: string[];
}

export type Constraints = Record<string, number>;

// front end storage for missions
export type ObjectiveDetail = ObjectiveResponse & Objective;
