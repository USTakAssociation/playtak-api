// Legacy games (before modern site usage). Constants/feature-flags so
// we can more easily trace this behavior.
// TODO: consider finally deleting some of this code.

// Timestamp playtak switched from anonymous to named accounts.
export const LEGACY_GAMES_CUTOFF = 1461430800000;

// Flip to false to stop masking pre-account player names as "Anon" (PTN export, name search).
// Plan: flip to false, get easier query plans.
export const LEGACY_GAMES_ANONYMIZED_FROM_RESULTS = false;

// Flip to false to include pre-account games in rating calculation.
// Plan: leave this in place unless we have reason to remove it.
export const LEGACY_GAMES_IGNORED_FOR_RATINGS = true;
