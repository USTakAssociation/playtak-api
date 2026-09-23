package tak.httpHandlers;

import com.sun.net.httpserver.HttpExchange;
import tak.DTOs.SeekDto;
import tak.GameSettings;
import tak.Player;
import tak.ProtocolFeature;
import tak.Seek;
import tak.exceptions.FailedToCreateSeekException;
import tak.exceptions.IncompatibleClientProtocolException;
import tak.exceptions.PlayerBusyWithGameException;
import tak.exceptions.PlayerNotFoundException;
import tak.exceptions.PlaytakException;

import java.io.IOException;
import java.util.Collection;
import java.util.logging.Level;

public class AddSeekHandler extends JsonHttpHandler {
	@Override
	public SeekDto PUT(HttpExchange t) throws IOException, FailedToCreateSeekException {
		try {
			SeekDto seekDto = jsonMapper.readValue(t.getRequestBody(), SeekDto.class);
			logger.log(Level.INFO, String.format("Successfully parsed DTO %s", seekDto.toString()));

			final Player creator = Player.getByName(seekDto.creator);

			if (creator == null || creator.client == null) {
				throw new PlayerNotFoundException(seekDto.creator);
			}
			if (creator.getGame() != null) {
				throw new PlayerBusyWithGameException(seekDto.creator);
			}
			// This endpoint is versionless, so it can otherwise post a seek on behalf of a
			// client connected at a protocol version that can't be told the seek's settings —
			// the creator would then desync in a game it never saw offered.
			final GameSettings requested = GameSettings.of(seekDto.openingCode(), seekDto.incrementScales);
			if (!ProtocolFeature.isCompatible(creator.client.protocolVersion, requested)) {
				throw new IncompatibleClientProtocolException(seekDto.creator, creator.client.protocolVersion,
					ProtocolFeature.requiredProtocolVersion(requested));
			}
			return Seek.newSeek(creator.client, seekDto).toDto();
		} catch (PlaytakException ex) {
			throw new FailedToCreateSeekException("Failed to create seek", ex);
		}
	}

	@Override
	public Collection<SeekDto> GET(HttpExchange httpExchange) {
		return Seek.getList();
	}
}
