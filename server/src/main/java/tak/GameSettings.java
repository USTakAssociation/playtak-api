package tak;

/**
 * The subset of a game's settings that the wire protocol gates on version.
 *
 * <p>Implemented by both {@link Seek} and {@link Game} so that
 * {@link ProtocolFeature} can ask a single question — "does this use a feature an
 * older client cannot be told about?" — of a seek and of the game it turns into,
 * and be guaranteed the same answer for both. Keeping one predicate rather than
 * one per type is deliberate: the seek-side and game-side serialisers have drifted
 * before (the opening code was appended in {@code Client.createGameStartString}
 * but forgotten in {@code Game.playerRejoin}), and a split predicate would let
 * that class of bug back in.
 *
 * <p>Add an accessor here only when a new setting is protocol-gated; see
 * {@link ProtocolFeature} for how to classify one.
 */
public interface GameSettings {
	/** Opening variant code. See {@link Opening}. */
	int opening();

	/** Whether the per-move increment is multiplied by the mover's move number. */
	boolean incrementScales();

	/**
	 * A standalone set of settings, for checking compatibility <em>before</em> a
	 * {@link Seek} or {@link Game} exists — e.g. when validating a seek request.
	 */
	static GameSettings of(int opening, boolean incrementScales) {
		return new Values(opening, incrementScales);
	}

	record Values(int opening, boolean incrementScales) implements GameSettings {
	}
}
