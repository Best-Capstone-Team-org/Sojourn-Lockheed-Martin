import type { ScenarioResponse } from "$lib/api";

class GameState {
	scenario: ScenarioResponse | undefined = $state(undefined);
}

export const gameState = new GameState();
