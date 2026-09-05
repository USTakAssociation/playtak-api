package tak;

/**
 * Registry of game settings that only newer protocol versions can express, and of
 * what it costs a client to not be told about them.
 *
 * <h2>Why a client can be safely under-served most of the time</h2>
 *
 * The server is authoritative for the clock and pushes <em>absolute</em> values to
 * both players and every spectator after each ply: {@code updateTimeTurnChange()}
 * ends in {@code sendTimeToAll()}, and {@code giveTime()} and the undo path do
 * the same. A client that knows nothing about the increment, increment scaling, extra
 * time or {@code GivenTime} therefore re-synchronises on the very next move.
 *
 * <p>The server is <em>not</em> authoritative for the board. There is no automatic
 * position resync anywhere — a client is sent the move stream and nothing else, and
 * models the position independently for the whole game. ({@code Show} is a manual,
 * human-readable dump; the move list replayed on join, rejoin and observe is still
 * just moves.) So a setting that changes how the move stream maps onto the board,
 * without changing the moves themselves, puts the client permanently and silently
 * out of step with the server.
 *
 * <p>That distinction — not "is the field missing?" — is what {@link Severity}
 * encodes, and it is why an old client can still be served the overwhelming majority
 * of games unharmed.
 *
 * <h2>Classifying a new feature</h2>
 *
 * When a setting becomes protocol-gated, add a constant here with the first protocol
 * version whose messages carry it, a predicate for whether a given game actually uses
 * it, and a severity. Ask: <em>if the client never learns this value, does its board
 * end up disagreeing with the server's?</em> If yes it is {@link Severity#DIVERGES}
 * and every gate below starts hiding and refusing those games automatically. If it
 * only affects the clock (self-correcting, above) or is pure metadata, it is
 * {@link Severity#DISPLAY_ONLY} and costs the client nothing but a label.
 *
 * <p>An audit of protocol versions 0-4 found exactly one {@code DIVERGES} setting,
 * {@link #OPENING_VARIANT}. Everything else withheld from an older client — the
 * increment itself below protocol 2, increment scaling and {@code GivenTime} below 4,
 * {@code is_bot}/{@code unrated}/{@code tournament} below 2 — is clock or label only,
 * and the game <em>result</em> never depended on protocol version at all.
 */
public enum ProtocolFeature {
	/**
	 * A non-default opening variant, carried as the trailing {@code opening} code from
	 * protocol 4.
	 *
	 * <p>{@code DIVERGES}: the move a Double Black Stack opening produces is byte-identical
	 * to an ordinary swap placement ({@code "P A1"}), so a client that was not told the
	 * opening renders one flat where the server holds two, with black's reserve off by one,
	 * and is never corrected. Every later ply compounds it — a spread out of that square
	 * moves a stone the client does not know exists, and the flat count that decides a
	 * flat win is wrong on one side.
	 */
	OPENING_VARIANT(4, Severity.DIVERGES) {
		@Override
		public boolean usedBy(GameSettings settings) {
			return settings.opening() != Opening.SWAP.code;
		}
	},

	/**
	 * Increment scaled by the mover's (1-indexed) move number, carried as the
	 * {@code scale_increment} flag from protocol 4.
	 *
	 * <p>{@code DISPLAY_ONLY}: purely a clock rule, and the server sends absolute clocks
	 * after every ply, so an older client re-synchronises each move and the result is
	 * never affected. Its locally animated countdown does run low between plies and then
	 * jump back up, which is ugly but harmless. Reclassifying this to {@code DIVERGES}
	 * would hide every scaled-increment seek from protocol &le; 3 clients.
	 */
	INCREMENT_SCALING(4, Severity.DISPLAY_ONLY) {
		@Override
		public boolean usedBy(GameSettings settings) {
			return settings.incrementScales();
		}
	};

	/** What it costs a client to never be told a feature's value. */
	public enum Severity {
		/** The client's board silently and permanently disagrees with the server's. */
		DIVERGES,
		/** Clock-only (self-correcting) or pure metadata; costs the client a label at most. */
		DISPLAY_ONLY
	}

	/** First protocol version whose messages carry this feature's value. */
	public final int minProtocolVersion;

	public final Severity severity;

	ProtocolFeature(int minProtocolVersion, Severity severity) {
		this.minProtocolVersion = minProtocolVersion;
		this.severity = severity;
	}

	/** Whether {@code settings} actually uses this feature (i.e. holds a non-default value). */
	public abstract boolean usedBy(GameSettings settings);

	/**
	 * The lowest protocol version that can play, watch or list {@code settings} without
	 * its board diverging from the server's. 0 for an ordinary game, which is the common
	 * case and stays visible to every client.
	 */
	public static int requiredProtocolVersion(GameSettings settings) {
		int required = 0;
		for (ProtocolFeature feature : values()) {
			if (feature.severity == Severity.DIVERGES && feature.usedBy(settings)) {
				required = Math.max(required, feature.minProtocolVersion);
			}
		}
		return required;
	}

	/**
	 * Whether a client speaking {@code clientProtocolVersion} can be shown, and allowed to
	 * join or observe, a seek or game with these settings.
	 *
	 * <p>Both inputs are immutable for as long as it matters: a client's version is parsed
	 * only in the pre-authentication branch of {@code Client.run()}, so it is frozen before
	 * the client ever registers as a seek or game-list listener; and a seek's and game's
	 * settings are fixed at construction. Every "add" broadcast this gate suppresses is
	 * therefore guaranteed to have its matching "remove" suppressed too, so no listener is
	 * ever left holding a stale row.
	 */
	public static boolean isCompatible(int clientProtocolVersion, GameSettings settings) {
		return clientProtocolVersion >= requiredProtocolVersion(settings);
	}
}
