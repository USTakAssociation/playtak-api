package tak.exceptions;

public class FailedToCreateSeekException extends PlaytakException {
	public FailedToCreateSeekException(String message, Exception innerException) {
		super(message, innerException);
	}

	@Override
	public String getMessage() {
		return super.getMessage() + ": " + this.getCause().getMessage();
	}
}
