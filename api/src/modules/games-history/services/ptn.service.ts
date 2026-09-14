export class PTNService {
	public getHeader(key: string, val: any) {
		return `[${key} "${val}"]\n`;
	}

	public convertMove(move: any) {
		const spl = move.split(' ');
		if (spl[0] === 'P') {
			// P A4 (C|W)
			const sq = spl[1];
			let stone = '';
			if (spl.length == 3) {
				stone = spl[2] == 'C' ? 'C' : 'S';
			}
			return stone + sq.toLowerCase();
		} else if (spl[0] === 'M') {
			// M A2 A5 2 1
			const fl1 = spl[1][0];
			const rw1 = spl[1][1];
			const fl2 = spl[2][0];
			const rw2 = spl[2][1];

			let dir = '';
			if (fl2 === fl1) {
				dir = rw2 > rw1 ? '+' : '-';
			} else {
				dir = fl2 > fl1 ? '>' : '<';
			}

			let lst = '';
			let liftsize = 0;
			for (let i = 3; i <= spl.length - 1; i++) {
				lst += spl[i];
				liftsize += parseInt(spl[i]);
			}

			return liftsize.toString() + spl[1].toLowerCase() + dir + lst;
		}

		return '';
	}

	public getMoves(notation: string, opening?: string, clocks?: number[] | null) {
		let moves = '';
		let count = 0;
		// A game with no moves recorded yet has an empty notation, and
		// ''.split(',') yields [''] rather than [], which ran the loop once and
		// emitted a move number with nothing after it. For a double black stack
		// game that produced a lone '2' prefix — "1. 2" — which is not valid PTN.
		const moveArray = this.splitNotation(notation);
		for (let i = 0; i < moveArray.length; i++) {
			const move = moveArray[i];
			if (count % 2 == 0) {
				moves += '\n' + (count / 2 + 1).toString() + '.';
			}

			moves += ' ';
			// Double Black Stack opening: White's first ply places two black flats,
			// written in PTN with a leading "2" (e.g. "2a1").
			if (i === 0 && opening === 'double black stack') {
				moves += '2';
			}
			moves += this.convertMove(move);

			if (clocks) {
				// White (Player1) makes every even-indexed ply.
				moves += ' ' + this.getClockComment(i % 2 === 0 ? 1 : 2, clocks[i]);
			}

			count += 1;
		}
		if (clocks && moveArray.length > 0) {
			// The trailing value is the clock of the player to move when the game
			// ended, so the final position shows what was left on it (e.g. 0:00
			// after a timeout, or the time remaining at a resignation).
			const playerToMove = moveArray.length % 2 === 0 ? 1 : 2;
			moves += ' ' + this.getClockComment(playerToMove, clocks[moveArray.length]);
		}
		return moves;
	}

	private splitNotation(notation: string) {
		return notation ? notation.split(',').filter((move) => move !== '') : [];
	}

	/**
	 * Parses the games.clocks column: comma-separated remaining milliseconds, one
	 * per ply for the player who made it, plus a final value for the player to move
	 * when the game ended. Returns null for games without usable clock data so the
	 * PTN is simply written without clocks.
	 */
	public parseClocks(clocks: string | null | undefined, notation: string): number[] | null {
		if (!clocks) {
			return null;
		}
		const values = clocks.split(',');
		if (
			values.length !== this.splitNotation(notation).length + 1 ||
			!values.every((value) => /^\d+$/.test(value))
		) {
			return null;
		}
		return values.map(Number);
	}

	/**
	 * A per-ply clock note in the form PTN Ninja records while spectating a PlayTak
	 * game, e.g. "{clock1:4:32}" or "{clock2:0:08.34}"; PTN Ninja replays these when
	 * stepping through the game.
	 */
	public getClockComment(player: 1 | 2, ms: number) {
		return `{clock${player}:${this.formatClockValue(ms)}}`;
	}

	// Mirrors PTN Ninja's formatClockNoteValue: "M:SS" or "H:MM:SS", with decimal
	// seconds (trailing zeros trimmed) only under a minute, where its timer shows them.
	public formatClockValue(ms: number) {
		const totalMs = Math.max(0, Math.round(ms));
		const totalSeconds = Math.floor(totalMs / 1000);
		const h = Math.floor(totalSeconds / 3600);
		const m = Math.floor((totalSeconds % 3600) / 60);
		const s = totalSeconds % 60;
		const pad = (n: number) => (n < 10 ? '0' + n : '' + n);
		let secStr = pad(s);
		if (totalMs < 60000) {
			const frac = totalMs % 1000;
			if (frac) {
				secStr += '.' + String(frac).padStart(3, '0').replace(/0+$/, '');
			}
		}
		return h > 0 ? `${h}:${pad(m)}:${secStr}` : `${m}:${secStr}`;
	}

	private formatDuration(totalSeconds: number) {
		const secs = totalSeconds % 60;
		totalSeconds = totalSeconds / 60;
		// Floored like hrs below: without this a duration that is not a whole
		// number of minutes rendered its minutes fractionally, e.g. 90 seconds
		// as "1.5:30" instead of "1:30".
		const mins = Math.floor(totalSeconds % 60);
		const hrs = Math.floor(totalSeconds / 60);
		let val = '';
		let force = false;

		if (hrs >= 1) {
			val += hrs.toString() + ':';
			force = true;
		}
		if (mins >= 1 || force) {
			val += mins.toString() + ':';
		}
		val += secs.toString();

		return val;
	}

	public getTimerInfo(
		timertime: number,
		timerinc: number,
		incrementScales = false,
		extraTimeTrigger = 0,
		extraTimeAmount = 0
	) {
		let val = this.formatDuration(timertime);

		if (timerinc !== 0) {
			// An increment that scales with the move number is written with a
			// trailing "n" ("+1n" = one second per move elapsed), matching how the
			// web client shows it. Without this the tag is identical to a fixed
			// increment, so the time control cannot be recovered from the PTN.
			val += ' +' + timerinc.toString() + (incrementScales ? 'n' : '');
		}

		if (extraTimeTrigger > 0 && extraTimeAmount > 0) {
			// Bonus time granted once the game reaches a given move, written as
			// "@move +duration" ("@35 +10:0" = ten minutes added at move 35).
			val += ' @' + extraTimeTrigger.toString() + ' +' + this.formatDuration(extraTimeAmount);
		}

		return val;
	}

	/**
	 * @param options.includeClocks append each ply's remaining clock as a PTN Ninja
	 *   clock note. Off by default so ordinary PTN exports stay compact; only
	 *   links that open the game in PTN Ninja ask for it.
	 */
	public getPTN(game: any, options: { includeClocks?: boolean } = {}) {
		let ptn = '';
		const wn = game.date < 1461430800000 ? 'Anon' : game.player_white;
		const wr = game.rating_white;
		const bn = game.date < 1461430800000 ? 'Anon' : game.player_black;
		const wb = game.rating_black;

		ptn += this.getHeader('Site', 'PlayTak.com');
		ptn += this.getHeader('Event', 'Online Play');

		let dt = new Date(game.date).toISOString();
		dt = dt.replace('T', ' ').split('.')[0];

		ptn += this.getHeader('Date', dt.split(' ')[0].replaceAll('-', '.'));
		ptn += this.getHeader('Time', dt.split(' ')[1]);

		ptn += this.getHeader('Player1', wn);
		if (wr) ptn += this.getHeader('Rating1', wr);
		ptn += this.getHeader('Player2', bn);
		if (wb) ptn += this.getHeader('Rating2', wb);
		ptn += this.getHeader(
			'Clock',
			this.getTimerInfo(
				game.timertime,
				game.timerinc,
				!!game.increment_scales,
				game.extra_time_trigger,
				game.extra_time_amount
			)
		);
		ptn += this.getHeader('Result', game.result);
		ptn += this.getHeader('Size', game.size);
		ptn += this.getHeader('Komi', (game.komi / 2).toString());

		const stdpieces = [0, 0, 0, 10, 15, 21, 30, 40, 50][game.size];
		const stdcaps = [0, 0, 0, 0, 0, 1, 1, 2, 2][game.size];
		const gpieces = game.pieces == -1 ? stdpieces : game.pieces;
		const gcaps = game.capstones == -1 ? stdcaps : game.capstones;

		ptn += this.getHeader('Flats', gpieces);
		ptn += this.getHeader('Caps', gcaps);

		// Opening variant (PTN Ninja tag). Omitted for the default "swap" so legacy PTN is unchanged.
		if (game.opening && game.opening !== 'swap') {
			ptn += this.getHeader('Opening', game.opening);
		}

		const clocks = options.includeClocks ? this.parseClocks(game.clocks, game.notation) : null;
		ptn += '\n' + this.getMoves(game.notation, game.opening, clocks);
		ptn += '\n' + game.result + '\n';

		return ptn;
	}
}
