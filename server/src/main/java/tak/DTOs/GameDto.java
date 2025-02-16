package tak.DTOs;

import java.util.UUID;

import lombok.Builder;
import lombok.Value;
import lombok.extern.jackson.Jacksonized;

@Value
@Builder
@Jacksonized
public class GameDto {
	public int id;
	/** PNT Game ID, relating to a tournament game */
	public Integer pntId;
	public String white;
	public String black;
	/** Seconds */
	public int timeContingent;
	/** Seconds */
	public int timeIncrement;
	public int extraTimeTriggerMove;
	/** Seconds */
	public int extraTimeAmount;
	public float komi;

	public int boardSize;
	public int capstones;
	public int pieces;
	public boolean unrated;
	public boolean tournament;

	/** Moves of the game, currently in Server Notation - should eventually be PTN */
	public String[] moves;
	/** Game result looking like F-0/1-0/R-0/etc. `null` if the game is not yet over. */
	public String result;

	public int tournamentInt() {
		return tournament ? 1 : 0;
	}

	public int unratedInt() {
		return unrated ? 1 : 0;
	}

	public int komiInt() {
		return Math.round(komi * 2);
	}
}
