/*
 * To change this license header, choose License Headers in Project Properties.
 * To change this template file, choose Tools | Templates
 * and open the template in the editor.
 */
package tak;

import tak.DTOs.SeekDto;
import tak.utils.ConcurrentHashSet;

import java.util.List;
import java.util.Map;
import java.util.concurrent.ConcurrentHashMap;
import java.util.concurrent.atomic.AtomicInteger;
import java.util.concurrent.locks.Lock;
import java.util.concurrent.locks.ReentrantLock;

import static tak.Game.DEFAULT_SIZE;

/**
 *
 * @author chaitu
 */
public class Seek {
	public enum COLOR {WHITE, BLACK, ANY}

	Client client;
	int boardSize;
	int no;
	/**
	 * The PNT GameId this Seek is for. Use `null` if not related to any PNT Game.
	 */
	final Integer pntId;
	int time;//time in seconds for each side
	int incr;//increment in seconds
	int komi;
	int pieces;
	int capstones;
	int unrated;
	int tournament;
	int triggerMove;
	int timeAmount;
	/**
	 * When true, the increment awarded after each move is multiplied by the
	 * (1-indexed) move number of the player who just moved.
	 */
	boolean incrementScales;
	String opponent;
	COLOR color;
	int botSeek;
	int rematchId;

	public static Lock seekStuffLock = new ReentrantLock(); // this locks out all other threads

	static AtomicInteger seekNo = new AtomicInteger(0);

	static Map<Integer, Seek> seeks = new ConcurrentHashMap<>();
	static ConcurrentHashSet<Client> seekListeners = new ConcurrentHashSet<>();

	/**
	 * Creates a new seek with the same settings as the given seek, but with a new `no`
	 */
	public static Seek newSeek(Client client, SeekDto seek) {
		return newSeek(client, seek.boardSize, seek.timeContingent, seek.timeIncrement, seek.color, seek.komiInt(), seek.pieces, seek.capstones, seek.unratedInt(), seek.tournamentInt(), seek.extraTimeTriggerMove, seek.extraTimeAmount, seek.incrementScales, seek.opponent, seek.pntId);
	}

	/** Backwards-compatible overload (incrementScales defaults to false). */
	public static Seek newSeek(Client client, int boardSize, int timeContingent, int timeIncrement, COLOR clr, int komi, int pieces, int capstones, int unrated, int tournament, int triggerMove, int timeAmount, String opponent, Integer pntId) {
		return newSeek(client, boardSize, timeContingent, timeIncrement, clr, komi, pieces, capstones, unrated, tournament, triggerMove, timeAmount, false, opponent, pntId);
	}

	public static Seek newSeek(Client client, int boardSize, int timeContingent, int timeIncrement, COLOR clr, int komi, int pieces, int capstones, int unrated, int tournament, int triggerMove, int timeAmount, boolean incrementScales, String opponent, Integer pntId) {
		opponent = opponent == null ? "" : opponent;

		seekStuffLock.lock();
		try {
			Seek sk = new Seek(client, boardSize, timeContingent, timeIncrement, clr, komi, pieces, capstones, unrated, tournament, triggerMove, timeAmount, incrementScales, opponent, pntId, -1);
			addSeek(sk);
			return sk;
		} finally {
			seekStuffLock.unlock();
		}
	}

	Seek(Client client, int boardSize, int timeContingent, int timeIncrement, COLOR clr, int komi, int pieces, int capstones, int unrated, int tournament, int triggerMove, int timeAmount, boolean incrementScales, String opponent, Integer pntId, int rematchId) {
		seekStuffLock.lock();
		try {
			this.client = client;
			no = seekNo.incrementAndGet();
			this.pntId = pntId;
			this.rematchId = rematchId;
			time = timeContingent;
			incr = timeIncrement;
			color = clr;
			this.komi = Math.min(komi, 8);
			this.pieces = Math.max(Math.min(pieces, 80), 10);
			this.capstones = Math.min(capstones, 5);
			this.unrated = unrated;
			this.tournament = tournament;
			this.triggerMove = triggerMove;
			this.timeAmount = timeAmount;
			this.incrementScales = incrementScales;
			this.opponent = opponent;
			if (client.player.isBot()) {
				this.botSeek = 1;
			} else {
				this.botSeek = 0;
			}
			if (boardSize < 3 || boardSize > 8) boardSize = DEFAULT_SIZE;
			this.boardSize = boardSize;
		} finally {
			seekStuffLock.unlock();
		}
	}

	static void removeSeek(int b) {
		seekStuffLock.lock();
		try {
			Seek sk = Seek.seeks.get(b);
			if (sk != null) {
				Seek.seeks.remove(b);
				updateListeners("remove ", sk.buildSeekStringArray());
			}
		} finally {
			seekStuffLock.unlock();
		}
	}

	static void addSeek(Seek sk) {
		seekStuffLock.lock();
		try {
			sk.client.removeSeeks();
			sk.client.seek = sk;
			Seek.seeks.put(sk.no, sk);
			updateListeners("new ", sk.buildSeekStringArray());
		} finally {
			seekStuffLock.unlock();
		}
	}

	/** Backwards-compatible overload (incrementScales defaults to false). */
	public static Seek newRematchSeek(Client c, int id, int boardSize, int time, int increment, String color, int komi, int pieces, int capstones, int unrated, int tournament, int triggerMove, int timeAmount, String opponent) {
		return newRematchSeek(c, id, boardSize, time, increment, color, komi, pieces, capstones, unrated, tournament, triggerMove, timeAmount, false, opponent);
	}

	public static Seek newRematchSeek(Client c, int id, int boardSize, int time, int increment, String color, int komi, int pieces, int capstones, int unrated, int tournament, int triggerMove, int timeAmount, boolean incrementScales, String opponent) {
		seekStuffLock.lock();
		try {
			COLOR colorEnum;
			if (color.equals("W")) {
				colorEnum = COLOR.WHITE;
			} else if (color.equals("B")) {
				colorEnum = COLOR.BLACK;
			} else {
				colorEnum = COLOR.ANY;
			}
			c.removeSeeks();
			Seek sk = new Seek(c, boardSize, time, increment, colorEnum, komi, pieces, capstones, unrated, tournament, triggerMove, timeAmount, incrementScales, opponent, -1, id);
			c.seek = sk;
			Seek.seeks.put(sk.no, sk);
			updateListeners("new ", sk.buildSeekStringArray());
			return sk;
		} finally {
			seekStuffLock.unlock();
		}
	}

	static void sendListTo(Client c) {
		seekStuffLock.lock();
		try {
			for (Integer no : Seek.seeks.keySet()) {
				String[] st = Seek.seeks.get(no).buildSeekStringArray();
				if (c.protocolVersion <= 1) {
					c.send("Seek new " + st[0]);
				} else if (c.protocolVersion == 2) {
					c.send("Seek new " + st[1]);
				} else {
					c.send("Seek new " + st[2]);
				}
			}
		} finally {
			seekStuffLock.unlock();
		}
	}

	public static List<SeekDto> getList() {
		seekStuffLock.lock();
		try {
			return Seek.seeks.values().stream().map(s -> s.toDto()).toList();
		} finally {
			seekStuffLock.unlock();
		}
	}

	public SeekDto toDto() {
		seekStuffLock.lock();
		try {
			return SeekDto.builder()
				.id(no)
				.pntId(pntId)
				.creator(client.player.getName())
				.opponent(opponent.equals("") ? null : opponent)
				.color(color)
				.komi(komi / 2.f)
				.boardSize(boardSize)
				.capstones(capstones)
				.pieces(pieces)
				.unrated(unrated > 0)
				.tournament(tournament > 0)
				.timeContingent(time)
				.timeIncrement(incr)
				.extraTimeAmount(timeAmount)
				.extraTimeTriggerMove(triggerMove)
				.incrementScales(incrementScales)
				.build();
		} finally {
			seekStuffLock.unlock();
		}
	}

	static void registerListener(Client c) {
		seekStuffLock.lock();
		try {
			seekListeners.add(c);
			sendListTo(c);
		} finally {
			seekStuffLock.unlock();
		}
	}

	static void updateListeners(String type, final String[] st) {
		seekStuffLock.lock();
		try {
			for (Client cc : seekListeners) {
				// check if the client protocol version
				if (cc.protocolVersion <= 1) {
					cc.sendWithoutLogging("Seek " + type + st[0]);
				} else if (cc.protocolVersion == 2) {
					cc.sendWithoutLogging("Seek " + type + st[1]);
				} else {
					cc.sendWithoutLogging("Seek " + type + st[2]);
				}
			}
		} finally {
			seekStuffLock.unlock();
		}
	}

	static void unregisterListener(Client c) {
		seekStuffLock.lock();
		try {
			seekListeners.remove(c);
		} finally {
			seekStuffLock.unlock();
		}
	}

	public String[] buildSeekStringArray() {
		seekStuffLock.lock();
		try {
			String clr = "A";
			if (color == COLOR.WHITE) clr = "W";
			else if (color == COLOR.BLACK) clr = "B";
			String playerName = client.player.getName();
			String v1Seek = String.join(" ", new String[] {
				Integer.toString(no),
				playerName,
				Integer.toString(boardSize),
				Integer.toString(time),
				Integer.toString(incr),
				clr,
				Integer.toString(komi),
				Integer.toString(pieces),
				Integer.toString(capstones),
				Integer.toString(unrated),
				Integer.toString(tournament),
				Integer.toString(triggerMove),
				Integer.toString(timeAmount),
				opponent
			});
			String v2Seek = String.join(" ", new String[] {
				Integer.toString(no),
				playerName,
				Integer.toString(boardSize),
				Integer.toString(time),
				Integer.toString(incr),
				clr,
				Integer.toString(komi),
				Integer.toString(pieces),
				Integer.toString(capstones),
				Integer.toString(unrated),
				Integer.toString(tournament),
				Integer.toString(triggerMove),
				Integer.toString(timeAmount),
				opponent.length() != 0 ? opponent : "0",
				client.player.isBot() ? "1" : "0"
			});
			String v3Seek = String.join(" ", new String[] {
				Integer.toString(no),
				playerName,
				Integer.toString(boardSize),
				Integer.toString(time),
				Integer.toString(incr),
				incrementScales ? "1" : "0",
				clr,
				Integer.toString(komi),
				Integer.toString(pieces),
				Integer.toString(capstones),
				Integer.toString(unrated),
				Integer.toString(tournament),
				Integer.toString(triggerMove),
				Integer.toString(timeAmount),
				opponent.length() != 0 ? opponent : "0",
				client.player.isBot() ? "1" : "0"
			});
			return new String[] {
				v1Seek,
				v2Seek,
				v3Seek
			};
		} finally {
			seekStuffLock.unlock();
		}
	}
}
