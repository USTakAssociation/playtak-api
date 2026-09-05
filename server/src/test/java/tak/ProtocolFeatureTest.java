package tak;

import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Nested;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.params.ParameterizedTest;
import org.junit.jupiter.params.provider.ValueSource;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertFalse;
import static org.junit.jupiter.api.Assertions.assertTrue;

class ProtocolFeatureTest {

	private static final GameSettings ORDINARY = GameSettings.of(Opening.SWAP.code, false);
	private static final GameSettings DOUBLE_BLACK_STACK = GameSettings.of(Opening.DOUBLE_BLACK_STACK.code, false);
	private static final GameSettings SCALING = GameSettings.of(Opening.SWAP.code, true);
	private static final GameSettings BOTH = GameSettings.of(Opening.DOUBLE_BLACK_STACK.code, true);

	@Nested
	@DisplayName("an ordinary game stays visible to every client")
	class OrdinaryGames {

		@ParameterizedTest
		@ValueSource(ints = {0, 1, 2, 3, 4, 5})
		void isCompatibleWithEveryProtocolVersion(int protocolVersion) {
			assertTrue(ProtocolFeature.isCompatible(protocolVersion, ORDINARY));
		}

		@Test
		void requiresNoParticularProtocolVersion() {
			assertEquals(0, ProtocolFeature.requiredProtocolVersion(ORDINARY));
		}
	}

	@Nested
	@DisplayName("a diverging feature is gated")
	class DivergingFeatures {

		@ParameterizedTest
		@ValueSource(ints = {0, 1, 2, 3})
		void doubleBlackStackIsHiddenFromOlderClients(int protocolVersion) {
			assertFalse(ProtocolFeature.isCompatible(protocolVersion, DOUBLE_BLACK_STACK));
		}

		@ParameterizedTest
		@ValueSource(ints = {4, 5})
		void doubleBlackStackIsServedToClientsThatCanRenderIt(int protocolVersion) {
			assertTrue(ProtocolFeature.isCompatible(protocolVersion, DOUBLE_BLACK_STACK));
		}

		@Test
		void requiredVersionIsTheVersionThatCarriesTheField() {
			assertEquals(4, ProtocolFeature.requiredProtocolVersion(DOUBLE_BLACK_STACK));
		}
	}

	@Nested
	@DisplayName("a display-only feature is not gated")
	class DisplayOnlyFeatures {

		/**
		 * The server pushes absolute clocks after every ply, so a client that was never told
		 * the increment scales re-synchronises each move. Gating on it would needlessly hide
		 * these seeks from every protocol <= 3 client, bots included.
		 */
		@ParameterizedTest
		@ValueSource(ints = {0, 1, 2, 3, 4})
		void incrementScalingIsServedToEveryClient(int protocolVersion) {
			assertTrue(ProtocolFeature.isCompatible(protocolVersion, SCALING));
		}

		@Test
		void incrementScalingDoesNotRaiseTheRequiredVersion() {
			assertEquals(0, ProtocolFeature.requiredProtocolVersion(SCALING));
		}
	}

	@Nested
	@DisplayName("the required version combines features")
	class CombinedFeatures {

		@Test
		void takesTheHighestDivergingFeatureAndIgnoresDisplayOnlyOnes() {
			assertEquals(4, ProtocolFeature.requiredProtocolVersion(BOTH));
			assertFalse(ProtocolFeature.isCompatible(3, BOTH));
			assertTrue(ProtocolFeature.isCompatible(4, BOTH));
		}
	}

	@Nested
	@DisplayName("registry invariants")
	class RegistryInvariants {

		@Test
		void everyFeatureIsUnusedByAnOrdinaryGame() {
			// The common case must stay ungated, whatever features get added later.
			for (ProtocolFeature feature : ProtocolFeature.values()) {
				assertFalse(feature.usedBy(ORDINARY),
					feature + " reports an ordinary swap game as using it, which would gate every game");
			}
		}

		@Test
		void everyDivergingFeatureIsGatedAboveProtocolZero() {
			// A DIVERGES feature at version 0 would be unenforceable — there is no older
			// version to withhold it from.
			for (ProtocolFeature feature : ProtocolFeature.values()) {
				if (feature.severity == ProtocolFeature.Severity.DIVERGES) {
					assertTrue(feature.minProtocolVersion > 0,
						feature + " diverges but claims to be expressible in protocol 0");
				}
			}
		}

		/**
		 * Severity is a deliberate judgement about whether an under-served client's board
		 * silently disagrees with the server's, not an incidental label. Changing one of
		 * these is a real behaviour change — the classification reasoning lives in
		 * {@link ProtocolFeature}'s javadoc, so read it before updating this test.
		 */
		@Test
		void classificationsAreTheOnesWeIntended() {
			assertEquals(ProtocolFeature.Severity.DIVERGES, ProtocolFeature.OPENING_VARIANT.severity);
			assertEquals(4, ProtocolFeature.OPENING_VARIANT.minProtocolVersion);

			assertEquals(ProtocolFeature.Severity.DISPLAY_ONLY, ProtocolFeature.INCREMENT_SCALING.severity);
			assertEquals(4, ProtocolFeature.INCREMENT_SCALING.minProtocolVersion);
		}
	}

	@Nested
	@DisplayName("feature predicates")
	class Predicates {

		@Test
		void openingVariantTracksNonDefaultOpenings() {
			assertFalse(ProtocolFeature.OPENING_VARIANT.usedBy(ORDINARY));
			assertTrue(ProtocolFeature.OPENING_VARIANT.usedBy(DOUBLE_BLACK_STACK));
		}

		@Test
		void incrementScalingTracksTheFlag() {
			assertFalse(ProtocolFeature.INCREMENT_SCALING.usedBy(ORDINARY));
			assertTrue(ProtocolFeature.INCREMENT_SCALING.usedBy(SCALING));
		}
	}
}
