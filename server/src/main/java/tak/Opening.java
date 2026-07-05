package tak;

/**
 * Game opening variant, controlling the special first-move ("swap") behaviour.
 *
 * <p>The string values match the PTN Ninja {@code [Opening "..."]} header tag so
 * they can be stored in the games DB and emitted as PTN with no translation.
 * {@code "no swap"} is a valid PTN Ninja value but is intentionally not
 * implemented yet; unknown/missing values fall back to {@link #SWAP}.
 */
public enum Opening {
	SWAP(0, "swap"),
	DOUBLE_BLACK_STACK(1, "double black stack");

	/** Compact code used on the space-delimited Seek wire protocol. */
	public final int code;
	/** Canonical PTN Ninja "Opening" tag value (stored in the DB / emitted as PTN). */
	public final String ptn;

	Opening(int code, String ptn) {
		this.code = code;
		this.ptn = ptn;
	}

	public static Opening fromCode(int code) {
		for (Opening o : values()) {
			if (o.code == code) return o;
		}
		return SWAP;
	}

	public static Opening fromPtn(String value) {
		if (value != null) {
			for (Opening o : values()) {
				if (o.ptn.equalsIgnoreCase(value)) return o;
			}
		}
		return SWAP;
	}
}
