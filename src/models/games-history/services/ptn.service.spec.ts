import { Test, TestingModule } from '@nestjs/testing';
import { PTNService } from './ptn.service';

describe('PTNService', () => {
	let service: PTNService;

	beforeEach(async () => {
		const module: TestingModule = await Test.createTestingModule({
			providers: [
				PTNService
			],
		}).compile();
		service = module.get<PTNService>(PTNService);
	});

	it('should be defined', () => {
		expect(service).toBeDefined();
	});
	
	describe('Get PTN Header', () => {
		it('should return the correct value', () => {
			const setup = service.getHeader("hello", "world")
			expect(setup).toEqual('[hello "world"]\n');
		})
	});
	
	describe('Convert Move', () => {
		it('should convert P move successfully', () => {
			const placeP = service.convertMove('P A5');
			const placeW = service.convertMove('P E3 W');
			const placeC = service.convertMove('P D3 C');
			expect(placeP).toEqual('a5');
			expect(placeW).toEqual('Se3');
			expect(placeC).toEqual('Cd3');
		})
		
		it('should convert M move successfully', () => {
			const moveUp = service.convertMove('M E3 E4 1');
			const moveDown = service.convertMove('M C4 C3 1');
			const moveLeft = service.convertMove('M D3 C3 1');
			const moveRight = service.convertMove('M D4 E4 1');
			expect(moveUp).toEqual('1e3+1');
			expect(moveDown).toEqual('1c4-1');
			expect(moveLeft).toEqual('1d3<1');
			expect(moveRight).toEqual('1d4>1');
		});
		
		it('should convert nothing if invalid move', () => {
			const setup = service.convertMove('');
			expect(setup).toEqual('');
		});
	})
	
	describe('Get Moves', () => {
		it('should return the move list', () => {
			const moveString =
				'P C5,P E3,P C3,P A4,P B2,P A3,P E4,P C1,P D2,P A5,P E1,P C4,P D3,P A1,P A2 W,M C4 C3 1,M D3 C3 1,P D4,M C3 C1 1 2,P C4,P E5,M D4 E4 1,M E3 E4 1,P C3,M E4 E5 3,P D4,P E2,P E3 W,P D3 C,P D5 W,M E5 E4 4,M E3 E4 1,M A2 A3 1,M E4 E2 3 2,M D3 E3 1,M C3 C2 1,M D2 C2 1,P B5,P D1,P E4 C';
			const movesConverted = `\n1. c5 e3\n2. c3 a4\n3. b2 a3\n4. e4 c1\n5. d2 a5\n6. e1 c4\n7. d3 a1\n8. Sa2 1c4-1\n9. 1d3<1 d4\n10. 3c3-12 c4\n11. e5 1d4>1\n12. 1e3+1 c3\n13. 3e4+3 d4\n14. e2 Se3\n15. Cd3 Sd5\n16. 4e5-4 1e3+1\n17. 1a2+1 5e4-32\n18. 1d3>1 1c3-1\n19. 1d2<1 b5\n20. d1 Ce4`;
			const moves = service.getMoves(moveString);
			expect(moves).toEqual(movesConverted);
		})
	})
	
	describe('Get Timer Info', () => {
		it('should return the correct string', () => {
			const timerS = service.getTimerInfo(30, 20);
			const timerM = service.getTimerInfo(180, 20);
			const timerH = service.getTimerInfo(3600, 20);
			expect(timerS).toEqual('30 +20');
			expect(timerM).toEqual('3:0 +20');
			expect(timerH).toEqual('1:0:0 +20');
		})
	})
	
	describe('Get PTN', () => {
		it('should return full PTN output', () => {
			const game1 = { id: 500123, date: 1653488350594, size: 7, player_white: 'Covault', player_black: 'Catalyst', notation: 'P A7,P D4,P E4,P A4,P D3,P B4,P D2,P D7,P D5,P D6,P D1,P C4 C,P E5,M C4 D4 1,P E6,P E7,P F6,P F7,P G7 C,M D4 E4 2,M G7 F7 1,M E7 E6 1,M F6 E6 1,M E4 E5 3,M E6 C6 1 2,P D4 C,P C7,P C4,P C5,P G4,P E4 C,P A5,P C3,M E5 D5 4,M C3 C4 1,M D5 C5 5,M C7 D7 1,M C5 C6 6,P E5,M C6 D6 7,M F7 E7 2,M D4 C4 1,P E3,M D6 E6 7,P D5', result: 'R-0', timertime: 180, timerinc: 20, rating_white: 1080, rating_black: 1105, unrated: 0, tournament: 0, komi: 0, pieces: 40, capstones: 2, rating_change_white: 164, rating_change_black: -66};
			const game2 = { id: 500121, date: 1461430700000, size: 7, player_white: 'Covault', player_black: 'Catalyst', notation: 'P A7,P D4,P E4,P A4,P D3,P B4,P D2,P D7,P D5,P D6,P D1,P C4 C,P E5,M C4 D4 1,P E6,P E7,P F6,P F7,P G7 C,M D4 E4 2,M G7 F7 1,M E7 E6 1,M F6 E6 1,M E4 E5 3,M E6 C6 1 2,P D4 C,P C7,P C4,P C5,P G4,P E4 C,P A5,P C3,M E5 D5 4,M C3 C4 1,M D5 C5 5,M C7 D7 1,M C5 C6 6,P E5,M C6 D6 7,M F7 E7 2,M D4 C4 1,P E3,M D6 E6 7,P D5', result: 'R-0', timertime: 180, timerinc: 20, rating_white: 1080, rating_black: 1105, unrated: 0, tournament: 0, komi: 0, pieces: -1, capstones: -1, rating_change_white: 164, rating_change_black: -66};
			const ptn1 = `[Site "PlayTak.com"]\n[Event "Online Play"]\n[Date "2022.05.25"]\n[Time "14:19:10"]\n[Player1 "Covault"]\n[Player2 "Catalyst"]\n[Clock "3:0 +20"]\n[Result "R-0"]\n[Size "7"]\n[Komi "0"]\n[Flats "40"]\n[Caps "2"]\n\n\n1. a7 d4\n2. e4 a4\n3. d3 b4\n4. d2 d7\n5. d5 d6\n6. d1 Cc4\n7. e5 1c4>1\n8. e6 e7\n9. f6 f7\n10. Cg7 2d4>2\n11. 1g7<1 1e7-1\n12. 1f6<1 3e4+3\n13. 3e6<12 Cd4\n14. c7 c4\n15. c5 g4\n16. Ce4 a5\n17. c3 4e5<4\n18. 1c3+1 5d5<5\n19. 1c7>1 6c5+6\n20. e5 7c6>7\n21. 2f7<2 1d4<1\n22. e3 7d6>7\n23. d5\nR-0\n`;
			const ptn2 = `[Site "PlayTak.com"]\n[Event "Online Play"]\n[Date "2016.04.23"]\n[Time "16:58:20"]\n[Player1 "Anon"]\n[Player2 "Anon"]\n[Clock "3:0 +20"]\n[Result "R-0"]\n[Size "7"]\n[Komi "0"]\n[Flats "40"]\n[Caps "2"]\n\n\n1. a7 d4\n2. e4 a4\n3. d3 b4\n4. d2 d7\n5. d5 d6\n6. d1 Cc4\n7. e5 1c4>1\n8. e6 e7\n9. f6 f7\n10. Cg7 2d4>2\n11. 1g7<1 1e7-1\n12. 1f6<1 3e4+3\n13. 3e6<12 Cd4\n14. c7 c4\n15. c5 g4\n16. Ce4 a5\n17. c3 4e5<4\n18. 1c3+1 5d5<5\n19. 1c7>1 6c5+6\n20. e5 7c6>7\n21. 2f7<2 1d4<1\n22. e3 7d6>7\n23. d5\nR-0\n`;
			const getPTN1 = service.getPTN(game1);
			const getPTN2 = service.getPTN(game2)
			expect(getPTN1).toEqual(ptn1);
			expect(getPTN2).toEqual(ptn2);
		})
	})
});
