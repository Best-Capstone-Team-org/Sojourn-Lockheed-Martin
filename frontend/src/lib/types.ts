import type { MissionStatusEnum } from "./enums";

export type MissionDetailType = {
	id: number;
	title: string;
	status: MissionStatusEnum;
	description: string;
	diagnostic?: string;
};