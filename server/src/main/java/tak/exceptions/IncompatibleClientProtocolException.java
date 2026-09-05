package tak.exceptions;

/**
 * Thrown when a seek would use a feature the creator's own connected client cannot be
 * told about, and so could not play or even see. See {@code tak.ProtocolFeature}.
 */
public class IncompatibleClientProtocolException extends PlaytakException {
	public IncompatibleClientProtocolException(String name, int clientProtocolVersion, int requiredProtocolVersion) {
		super(String.format(
			"Player '%s' is connected with protocol version %d, but this seek requires protocol version %d",
			name, clientProtocolVersion, requiredProtocolVersion));
	}
}
