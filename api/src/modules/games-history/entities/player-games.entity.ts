import { ViewColumn, ViewEntity } from 'typeorm';
import { Games } from './games.entity';

// The `player_games` view (scripts/migrations/2026-08-add-player-games-view.sh):
// every game flattened to one row per player, so a player search is
// `player_name = ?` with no OR. Read-only; created by the migration, not synced.
@ViewEntity({ name: 'player_games', synchronize: false })
export class PlayerGames extends Games {
	@ViewColumn()
	player_name: string;
}
