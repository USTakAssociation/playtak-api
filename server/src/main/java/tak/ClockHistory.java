package tak;

import java.util.List;
import java.util.StringJoiner;

/**
 * Encodes a finished game's clocks for the {@code games.clocks} column.
 *
 * <p>The column holds comma-separated remaining times in milliseconds, aligned
 * with {@code games.notation}: one value per ply for the player who made it,
 * taken immediately after the ply (so any increment or trigger-move bonus is
 * already applied), followed by one final value for the player who was to move
 * when the game ended. A game with no plies stores an empty string.
 *
 * <p>White makes every even-indexed ply (0, 2, 4, ...) regardless of opening,
 * so the mover never has to be stored.
 */
final class ClockHistory {

	private ClockHistory() {
	}

	/**
	 * @param clocksAfterPly {@code {whiteMs, blackMs}} as they stood immediately after each ply, in ply order
	 * @param endOfGameMs    remaining time of the player to move when the game ended
	 */
	static String encode(List<long[]> clocksAfterPly, long endOfGameMs) {
		if (clocksAfterPly.isEmpty()) {
			return "";
		}
		StringJoiner joiner = new StringJoiner(",");
		for (int ply = 0; ply < clocksAfterPly.size(); ply++) {
			long[] clocks = clocksAfterPly.get(ply);
			joiner.add(Long.toString(clamp(ply % 2 == 0 ? clocks[0] : clocks[1])));
		}
		joiner.add(Long.toString(clamp(endOfGameMs)));
		return joiner.toString();
	}

	// A player's clock has gone below zero by the time their timeout fires.
	private static long clamp(long ms) {
		return Math.max(ms, 0L);
	}
}
