package tak.exceptions;

public class PlaytakException extends Exception {
	public PlaytakException(String message) {
		super(message);
	}

	public PlaytakException(String message, Exception innerException) {
		super(message, innerException);
	}
}
