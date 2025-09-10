package tak.exceptions;

public class PlayerBusyWithGameException extends PlaytakException {
	public PlayerBusyWithGameException(String name) {
		super(String.format("Player '%s' is currently playing a game", name));
	}
}
