package tak;

import java.io.BufferedReader;
import java.io.InputStreamReader;
import java.io.PrintStream;
import java.sql.DriverManager;
import java.sql.Statement;

/**
 * Drives the Java server's Game class for the Go engine's differential test
 * (internal/tak/javadiff_test.go). It lives in package tak so it can reach
 * Game's package-private members.
 *
 * Commands, one per line:
 *   NEW size pieces capstones komi opening
 *   P w|b FILE RANK cap(0|1) wall(0|1)
 *   M w|b FILE RANK FILE RANK drop...
 *   U                (both players agree to undo)
 *
 * After NEW it prints READY. After every other command it prints three lines:
 *   OK | ERR message
 *   STATE gameState moveCount whiteStones whiteCaps blackStones blackCaps
 *   BOARD square|square|...   (rank 1 to N, file A to N within each rank)
 *   MOVES move,move,...
 */
public class DiffHarness {
	public static void main(String[] args) throws Exception {
		// Game prints diagnostics to System.out; keep our replies separate.
		PrintStream out = new PrintStream(System.out, false, "UTF-8");
		System.setOut(System.err);

		// Game writes its result to the games DB when it ends.
		Database.gamesConnection = DriverManager.getConnection("jdbc:sqlite::memory:");
		try (Statement st = Database.gamesConnection.createStatement()) {
			st.execute("CREATE TABLE games (id INTEGER PRIMARY KEY, notation TEXT, result TEXT)");
		}

		BufferedReader in = new BufferedReader(new InputStreamReader(System.in, "UTF-8"));
		Game g = null;
		int players = 0;
		String line;
		while ((line = in.readLine()) != null) {
			String[] f = line.split(" ");
			switch (f[0]) {
				case "NEW": {
					Player p1 = new Player("p" + (++players), "", "", false);
					Player p2 = new Player("p" + (++players), "", "", false);
					// With COLOR.WHITE the seek's poster (p2) plays white.
					g = new Game(p1, p2, Integer.parseInt(f[1]), 1_000_000, 0, Seek.COLOR.WHITE,
						Integer.parseInt(f[4]), Integer.parseInt(f[2]), Integer.parseInt(f[3]), 0, 0, 0, 0, null);
					g.opening = Integer.parseInt(f[5]);
					out.println("READY");
					break;
				}
				case "P": {
					Status s = g.placeMove(player(g, f[1]), f[2].charAt(0), Integer.parseInt(f[3]),
						f[4].equals("1"), f[5].equals("1"));
					report(out, g, s.isOk(), s.msg());
					break;
				}
				case "M": {
					int[] vals = new int[f.length - 6];
					for (int i = 6; i < f.length; i++) vals[i - 6] = Integer.parseInt(f[i]);
					Status s = g.moveMove(player(g, f[1]), f[2].charAt(0), Integer.parseInt(f[3]),
						f[4].charAt(0), Integer.parseInt(f[5]), vals);
					report(out, g, s.isOk(), s.msg());
					break;
				}
				case "U": {
					int before = g.board.moveCount;
					g.undo(g.white);
					g.undo(g.black);
					report(out, g, g.board.moveCount < before, "undo refused");
					break;
				}
				default:
					throw new IllegalArgumentException("unknown command " + line);
			}
			out.flush();
		}
		// Game schedules clock timers on a non-daemon thread.
		System.exit(0);
	}

	static Player player(Game g, String who) {
		return who.equals("w") ? g.white : g.black;
	}

	static void report(PrintStream out, Game g, boolean ok, String msg) {
		out.println(ok ? "OK" : "ERR " + msg);
		out.println("STATE " + g.gameState + " " + g.board.moveCount + " " + g.board.whiteTilesCount + " "
			+ g.board.whiteCapstones + " " + g.board.blackTilesCount + " " + g.board.blackCapstones);
		StringBuilder board = new StringBuilder("BOARD ");
		for (int rank = 1; rank <= g.board.boardSize; rank++) {
			for (int file = 0; file < g.board.boardSize; file++) {
				if (rank > 1 || file > 0) board.append('|');
				board.append(g.sqState((char) ('A' + file), rank));
			}
		}
		out.println(board);
		out.println("MOVES " + String.join(",", g.moveList));
	}
}
