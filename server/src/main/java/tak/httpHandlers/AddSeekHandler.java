package tak.httpHandlers;

import java.io.IOException;
import java.util.Collection;
import java.util.logging.Level;
import java.util.zip.DataFormatException;

import com.sun.net.httpserver.HttpExchange;

import tak.Player;
import tak.Seek;
import tak.DTOs.SeekDto;
import tak.exceptions.FailedToCreateSeekException;
import tak.exceptions.PlayerBusyWithGameException;
import tak.exceptions.PlayerNotFoundException;
import tak.exceptions.PlaytakException;

public class AddSeekHandler extends JsonHttpHandler {
	@Override
	public SeekDto PUT(HttpExchange t) throws IOException, DataFormatException, FailedToCreateSeekException {
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
