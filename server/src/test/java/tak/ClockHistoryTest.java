package tak;

import org.junit.jupiter.api.Test;

import java.util.List;

import static org.junit.jupiter.api.Assertions.assertEquals;

class ClockHistoryTest {

	@Test
	void aGameWithNoPliesStoresNothing() {
		assertEquals("", ClockHistory.encode(List.of(), 600_000));
	}

	@Test
	void eachPlyRecordsTheClockOfThePlayerWhoMadeIt() {
		List<long[]> clocks = List.of(
				new long[]{605_000, 600_000}, // white's ply 1
				new long[]{605_000, 598_250}, // black's ply 1
				new long[]{590_125, 598_250}  // white's ply 2
		);
		assertEquals("605000,598250,590125,597000", ClockHistory.encode(clocks, 597_000));
	}

	@Test
	void theFinalValueIsTheClockOfThePlayerToMove() {
		List<long[]> clocks = List.of(
				new long[]{605_000, 600_000},
				new long[]{605_000, 598_250}
		);
		// White is to move after two plies; white resigned with 12.5s left.
		assertEquals("605000,598250,12500", ClockHistory.encode(clocks, 12_500));
	}

	@Test
	void aClockThatRanOutIsStoredAsZero() {
		List<long[]> clocks = List.<long[]>of(new long[]{5_000, 3_000});
		assertEquals("5000,0", ClockHistory.encode(clocks, -42));
	}
}
