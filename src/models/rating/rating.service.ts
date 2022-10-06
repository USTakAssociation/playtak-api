import { Injectable } from '@nestjs/common';
import { Cron } from '@nestjs/schedule';
// var fs = require('fs');
// var crypto = require('crypto');
// var bsqlite3 = require('better-sqlite3');
// var zlib = require('zlib');
@Injectable()
export class RatingService {
	//constructor(){}

	@Cron('30 * * * * *')
	public handleRerate() {
		//Rating calculation parameters:
		const INITIAL_RATING = 1000;
		const BONUS_RATING = 750;
		const BONUS_FACTOR = 60;
		const PARTICIPATION_LIMIT = 10;
		const PARTICIPATION_CUTOFF = 1500;
		const MAX_DROP = 200;
		const RATING_RETENTION = 1000 * 60 * 60 * 24 * 240;
		const now = Date.now();
		const RECENT_LIMIT = now - 1000 * 60 * 60 * 6;
		//const source = fs.readFileSync('rerate.js');
		//const hash = crypto.createHash('sha256');
		//hash.update(source);
		//const sourcehash = hash.digest('hex');
		const sourcehash = '0';
		const previous = '0 0';
		try {
			//previous = fs.readFileSync('previous.txt', 'utf8');
		} catch (e) {}
		const previousdata = previous.split(' ');

		let incremental = false;
		let lastusedgame = 0;
		if (sourcehash == previousdata[1]) {
			incremental = true;
			lastusedgame = +previousdata[0];
		}

		let gameData;
		let playerData;
		//let ratinglistpath;
		// games = bsqlite3('/server/games.db', { fileMustExist: true });
		// players = bsqlite3('/server/players.db', { fileMustExist: true });
		// ratinglistpath = '/static/ratinglist.json.gz';

		const sameaccounts = {
			alphabot: 'alphatak_bot',
			TakticianBotDev: 'TakticianBot',
			sectenor: 'Turing',
			KingSultan: 'SultanPepper',
			PrinceSultan: 'SultanPepper',
			SultanTheGreat: 'SultanPepper',
			FuhrerSultan: 'SultanPepper',
			MaerSultan: 'SultanPepper',
			tarontos: 'Tarontos',
			Luffy: 'Ally',
			Archerion2: 'Archerion',
			Manet: 'Simmon',
			Alexc997: 'Doodles',
			DragonTakerDG: 'dylandragon',
			Bullet: 'Abyss',
			Saemon: 'Syme',
			Alpha: 'Syme',
			pse711933: 'LuKAs',
			rossin: 'archvenison',
			megafauna: 'archvenison',
			kriTakBot: 'TakkenBot',
			robot: 'TakkenBot',
		};
		const bots = [
			'TakticianBot',
			'alphatak_bot',
			'alphabot',
			'cutak_bot',
			'TakticianBotDev',
			'takkybot',
			'ShlktBot',
			'AlphaTakBot_5x5',
			'BeginnerBot',
			'TakkerusBot',
			'IntuitionBot',
			'AaaarghBot',
			'kriTakBot',
			'TakkenBot',
			'robot',
			'TakkerBot',
			'Geust93',
			'CairnBot',
			'VerekaiBot1',
			'BloodlessBot',
			'Tiltak_Bot',
			'Taik',
			'FlashBot',
			'FriendlyBot',
			'FPABot',
			'sTAKbot1',
			'sTAKbot2',
			'DoubleStackBot',
			'antakonistbot',
			'CrumBot',
		];
		const excluded = [
			'FlashBot',
			'FriendlyBot',
			'FPABot',
			'cutak_bot',
			'sTAKbot1',
			'sTAKbot2',
			'DoubleStackBot',
			'antakonistbot',
			'CairnBot',
			'MatthewHtn',
		];

		if (!incremental) {
			playerData.transaction(accountsettings)();
		}

		const pn = {}; // player names
		const pi = {}; // player id's
		let players; // players
		let games; // games

		if (incremental) {
			players = playerData
				.prepare(
					'select id,name,ratingbase,unrated,isbot,rating,boost,ratedgames,maxrating,ratingage,fatigue from players;',
				)
				.all();

			for (let i = 0; i < players.length; i++) {
				pn['!' + players[i].name] = players[i];
				pi[players[i].id] = players[i];
				players[i].fatigue = JSON.parse(players[i].fatigue);
				players[i].changed = false;
			}
		} else {
			players = playerData
				.prepare(
					'select id,name,ratingbase,unrated,isbot from players;',
				)
				.all();
			for (let i = 0; i < players.length; i++) {
				pn['!' + players[i].name] = players[i];
				pi[players[i].id] = players[i];
				players[i].rating = INITIAL_RATING;
				players[i].boost = BONUS_RATING;
				players[i].ratedgames = 0;
				players[i].maxrating = INITIAL_RATING;
				players[i].ratingage = 0;
				players[i].fatigue = {};
				players[i].changed = true;
				if (players[i].unrated) {
					players[i].rating = 0;
					players[i].maxrating = 0;
				}
			}
		}

		const updategame = gameData.prepare(
			'update games set rating_white=?, rating_black=?, rating_change_white=?, rating_change_black=? where id=?;',
		);
		let updating = true;

		let count;
		do {
			games = gameData
				.prepare(
					'select id,date,player_white,player_black,result,unrated,size,timertime,timerinc,pieces,capstones,length(notation) as notationlength from games where date>1461430800000 and id>? order by id asc limit 50000;',
				)
				.all(lastusedgame);
			gameData.transaction(handlegames)();
			count = games.length;
			games = null;
			if (global.gc) {
				global.gc();
			}

			Atomics.wait(new Int32Array(new SharedArrayBuffer(4)), 0, 0, 500);
		} while (count && updating);

		const updateplayer = playerData.prepare(
			'update players set rating=?, boost=?, ratedgames=?, maxrating=?, ratingage=?, fatigue=? where id=?;',
		);

		const alternames = {};

		playerData.transaction(handleplayers)();

		//fs.writeFileSync("previous.txt",lastgameid+" "+sourcehash)
		// TODO save last game used somewhere
		//fs.writeFileSync('previous.txt', lastusedgame + ' ' + sourcehash);

		// const playerlist = [];
		// for (let i = 0; i < players.length; i++) {
		// 	const player = players[i];
		// 	if (player.ratedgames > 0 || (player.unrated && player.isbot)) {
		// 		playerlist.push([
		// 			player.name +
		// 				(alternames.hasOwnProperty(player.id)
		// 					? alternames[player.id]
		// 					: ''),
		// 			adjustedrating(player, now),
		// 			Math.floor(player.rating),
		// 			player.ratedgames,
		// 			player.isbot ? 1 : 0,
		// 		]);
		// 	}
		// }
		// playerlist.sort(function (b, a) {
		// 	return a[1] - b[1];
		// });
		// for (let i = 0; i < playerlist.length; i++) {
		// 	playerlist[i][1] = Math.floor(playerlist[i][1]);
		// }
		//var jsonratinglist = JSON.stringify(playerlist);
		//var gzratinglist = zlib.gzipSync(jsonratinglist, { level: 9 });
		//fs.writeFileSync(ratinglistpath, gzratinglist);

		function getplayer(name) {
			let player = pn['!' + name];
			if (player && player.ratingbase > 0) {
				player = pi[player.ratingbase];
			}
			if (player && player.unrated == 1) {
				return null;
			}
			return player || null;
		}

		function handlegames() {
			for (let i = 0; i < games.length; i++) {
				const game = games[i];
				const playerWhite = getplayer(game.player_white);
				const playerBlack = getplayer(game.player_black);
				let rtw = 0; // player white rating
				let rtb = 0; // player black rating
				let artw = 0; // player white adjusted rating
				let artb = 0; // player balck adjusted rating
				if (playerWhite) {
					rtw = playerWhite.rating;
					artw = adjustedrating(playerWhite, game.date);
				}
				if (playerBlack) {
					rtb = playerBlack.rating;
					artb = adjustedrating(playerBlack, game.date);
				}
				const quickresult = {
					'R-0': 1,
					'F-0': 1,
					'1-0': 1,
					'0-R': 0,
					'0-F': 0,
					'0-1': 0,
					'1/2-1/2': 0.5,
				}[game.result];

				if (
					playerWhite &&
					playerBlack &&
					gameeligible(game) &&
					game.unrated == 0 &&
					playerWhite != playerBlack
				) {
					if (updating) {
						if (game.notationlength > 6) {
							lastusedgame = game.id;
							if (quickresult === undefined) {
								updategame.run(
									Math.floor(artw),
									Math.floor(artb),
									-2000,
									-2000,
									game.id,
								);
							} else {
								const sw = Math.pow(10, rtw / 400);
								const sb = Math.pow(10, rtb / 400);
								const expected = sw / (sw + sb);
								const fairness = expected * (1 - expected);
								const fatiguefactor =
									(1 - (playerWhite.fatigue[playerBlack.id] || 0) * 0.4) *
									(1 - (playerBlack.fatigue[playerWhite.id] || 0) * 0.4);
								adjustplayer(
									playerWhite,
									quickresult - expected,
									fairness,
									fatiguefactor,
									game.date,
								);
								adjustplayer(
									playerBlack,
									expected - quickresult,
									fairness,
									fatiguefactor,
									game.date,
								);
								updatefatigue(
									playerWhite,
									playerBlack.id,
									fairness * fatiguefactor,
								);
								updatefatigue(
									playerBlack,
									playerWhite.id,
									fairness * fatiguefactor,
								);
								const artw2 = adjustedrating(playerWhite, game.date);
								const artb2 = adjustedrating(playerBlack, game.date);
								updategame.run(
									Math.floor(artw),
									Math.floor(artb),
									Math.round((artw2 - artw) * 10),
									Math.round((artb2 - artb) * 10),
									game.id,
								);
							}
						} else {
							if (
								quickresult === undefined &&
								game.date > RECENT_LIMIT
							) {
								updating = false;
							} else {
								lastusedgame = game.id;
								updategame.run(
									Math.floor(artw),
									Math.floor(artb),
									-2000,
									-2000,
									game.id,
								);
							}
						}
					}
				} else {
					if (updating) {
						lastusedgame = game.id;
					}
					updategame.run(
						Math.floor(artw),
						Math.floor(artb),
						-2000,
						-2000,
						game.id,
					);
				}
			}
		}

		function gameeligible(game) {
			let eligible = game.size >= 5;
			const limits = [
				null,
				null,
				null,
				null,
				null,
				[180, 20, 32, 1, 1],
				[240, 25, 40, 1, 2],
				[300, 30, 48, 1, 2],
				[360, 40, 64, 1, 3],
			][game.size] || [180, 20, 32, 1, 1];
			if (game.pieces != -1) {
				eligible = eligible && game.pieces >= limits[1];
				eligible = eligible && game.pieces <= limits[2];
				eligible = eligible && game.capstones >= limits[3];
				eligible = eligible && game.capstones <= limits[4];
			}
			if (game.timertime > 0) {
				eligible =
					eligible &&
					limits[0] * 3 <=
						game.timertime * 3 + game.timerinc * limits[0];
				eligible = eligible && game.timertime >= 60;
			}
			return eligible;
		}

		// Process players
		function handleplayers() {
			for (let i = 0; i < players.length; i++) {
				if (players[i].changed) {
					updateplayer.run(
						players[i].rating,
						players[i].boost,
						players[i].ratedgames,
						players[i].maxrating,
						players[i].ratingage,
						JSON.stringify(players[i].fatigue),
						players[i].id,
					);
				}
				if (players[i].ratingbase) {
					if (alternames.hasOwnProperty(players[i].ratingbase)) {
						alternames[players[i].ratingbase] += ' ' + players[i].name;
					} else {
						alternames[players[i].ratingbase] = ' ' + players[i].name;
					}
				}
			}
		}
		// Adjust player rating
		function adjustplayer(player, amount, fairness, fatiguefactor, date) {
			const bonus = Math.min(
				Math.max(
					0, 
					(fatiguefactor * amount * Math.max(player.boost, 1) * BONUS_FACTOR) /
					BONUS_RATING,
				),
				player.boost,
			);
			player.boost -= bonus;
			const k =
				10 +
				15 * Math.pow(0.5, player.ratedgames / 200) +
				15 * Math.pow(0.5, (player.maxrating - INITIAL_RATING) / 300);
			player.rating += fatiguefactor * amount * k + bonus;
			if (player.ratingage == 0) {
				player.ratingage = date - RATING_RETENTION;
			}
			let participation =
				20 * Math.pow(0.5, (date - player.ratingage) / RATING_RETENTION);
			participation += fairness * fatiguefactor;
			participation = Math.min(20, participation);
			player.ratingage =
				Math.log2(participation / 20) * RATING_RETENTION + date;
			player.ratedgames++;
			player.maxrating = Math.max(player.maxrating, player.rating);
			player.changed = true;
		}

		function updatefatigue(player, opid, gamefactor) {
			const multiplier = 1 - gamefactor * 0.4;
			for (const i in player.fatigue) {
				player.fatigue[i] *= multiplier;
				if (i != opid && player.fatigue[i] < 0.01) {
					delete player.fatigue[i];
				}
			}
			player.fatigue[opid] = (player.fatigue[opid] || 0) + gamefactor;
		}

		function adjustedrating(player, date) {
			if (player.rating < PARTICIPATION_CUTOFF) {
				return player.rating;
			}
			const participation =
				20 * Math.pow(0.5, (date - player.ratingage) / RATING_RETENTION);
			if (player.rating < PARTICIPATION_CUTOFF + MAX_DROP) {
				return Math.min(
					player.rating,
					PARTICIPATION_CUTOFF +
						(MAX_DROP * participation) / PARTICIPATION_LIMIT,
				);
			} else {
				return Math.min(
					player.rating,
					player.rating -
						MAX_DROP * (1 - participation / PARTICIPATION_LIMIT),
				);
			}
		}

		function accountsettings() {
			const alias = playerData.prepare(
				'update players as a set ratingbase=(select b.id from players as b where b.name=?) where a.name=?;',
			);
			for (const key in sameaccounts) {
				alias.run(sameaccounts[key], key);
			}
			const setbot = playerData.prepare(
				'update players set isbot=1 where name=?;',
			);
			for (let i = 0; i < bots.length; i++) {
				setbot.run(bots[i]);
			}
			const setnorate = playerData.prepare(
				'update players set unrated=1 where name=?;',
			);
			for (let i = 0; i < excluded.length; i++) {
				setnorate.run(excluded[i]);
			}
		}
	}
}
