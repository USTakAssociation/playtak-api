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

	public getMoves(notation: string, opening?: string) {
		let moves = '';
		let count = 0;
		// A game with no moves recorded yet has an empty notation, and
		// ''.split(',') yields [''] rather than [], which ran the loop once and
		// emitted a move number with nothing after it. For a double black stack
		// game that produced a lone '2' prefix — "1. 2" — which is not valid PTN.
		const moveArray = notation ? notation.split(',').filter((move) => move !== '') : [];
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

			count += 1;
		}
		return moves;
	}

	public getTimerInfo(timertime: number, timerinc: number, incrementScales = false) {
		const secs = timertime % 60;
		timertime = timertime / 60;
		const mins = timertime % 60;
		const hrs = Math.floor(timertime / 60);
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
		if (timerinc !== 0) {
			// An increment that scales with the move number is written with a
			// trailing "n" ("+1n" = one second per move elapsed), matching how the
			// web client shows it. Without this the tag is identical to a fixed
			// increment, so the time control cannot be recovered from the PTN.
			val += ' +' + timerinc.toString() + (incrementScales ? 'n' : '');
		}

		return val;
	}

	public getPTN(game: any) {
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
		ptn += this.getHeader('Clock', this.getTimerInfo(game.timertime, game.timerinc, !!game.increment_scales));
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

		ptn += '\n' + this.getMoves(game.notation, game.opening);
		ptn += '\n' + game.result + '\n';

		return ptn;
	}
}
