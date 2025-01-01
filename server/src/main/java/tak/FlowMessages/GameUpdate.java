package tak.FlowMessages;

import lombok.NonNull;
import lombok.Value;
import tak.DTOs.GameDto;

@Value
public class GameUpdate {
	public static final String prefix = "game";
	public static final String created = prefix + ".created";
	public static final String ended = prefix + ".ended";

	public static GameUpdate gameCreated(GameDto game) {
		return new GameUpdate(GameUpdate.created, game);
	}

	public static GameUpdate gameEnded(GameDto game) {
		return new GameUpdate(GameUpdate.ended, game);
	}

	public final String type;
	@NonNull
	public final GameDto game;

	protected GameUpdate(String type, GameDto game) {
		this.type = type;

		assert game != null;
		this.game = game;
	}
}
