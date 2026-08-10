#!/bin/bash
# Migration script to add performance indexes to the databases
# This should be run against the games.db and players.db SQLite databases
# Usage:
#   sqlite3 /path/to/games.db < this_file    (for games indexes)
#   sqlite3 /path/to/players.db < this_file   (for players indexes)

-- Games database indexes
-- Create indexes on commonly queried columns to improve query performance
CREATE INDEX IF NOT EXISTS idx_games_date ON games(date);
-- Composite index for the most common search pattern (player searches)
CREATE INDEX IF NOT EXISTS idx_games_player_date ON games(player_white, player_black, date);
-- Mirror of the above for player_black-only lookups (UI defaults mirror=true,
-- and player_black-only + mirror is a real click path via the black player link)
CREATE INDEX IF NOT EXISTS idx_games_player_black_date ON games(player_black, date);

-- Players database indexes
-- Index on name column for player lookups
CREATE INDEX IF NOT EXISTS idx_players_name ON players(name);

