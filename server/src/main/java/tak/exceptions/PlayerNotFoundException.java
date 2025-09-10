package tak.exceptions;

public class PlayerNotFoundException extends PlaytakException {
	public PlayerNotFoundException(String name) {
		super(String.format("There is no player named '%s' connected", name));
	}
}
