# Walkthrough – v1.1.18 Client-Tier Tuning UI

## Summary
Added a **Client-Tier Tuning** section to the Performance View that recommends DNS resolver configuration based on subscriber count (client density), complementing the existing hardware-based profile system.

## Version
- **v1.1.18** | Commit `2501451` | Tag `v1.1.18`
- Previous: v1.1.17 (backend client-tier model)

## What Changed

### New Files
| File | Purpose |
|------|---------|
| `web/src/constants/clientTier.ts` | Tier thresholds (≤500 Small, ≤2000 Medium, >2000 Large), `ProfileConfig` type, `TIER_PROFILES` static data, `getClientTier()` utility |
| `web/src/styles/designTokens.css` | CSS variables for gradients, glass-morphism `.glass-card`, `.tier-badge` pulse animation |
| `web/src/components/ClientCountInput.tsx` | Numeric input + range slider with 300ms debounce |
| `web/src/components/TierBadge.tsx` | Gradient pill badge showing current tier |
| `web/src/components/TierProfileCard.tsx` | Glass-morphic card with tier info and Select button |

### Modified Files
| File | Change |
|------|--------|
| `web/src/views/PerformanceView.tsx` | +5 imports, +4 state vars, +68 lines JSX (client-tier section inserted between Disk Setup and Profile Selector Cards) |
| `web/src/types.ts` | Added `client_count?: number` to `DesiredState`, exported `ClientTier` type |
| `version.json` / `cmd/netora-api/version.go` | Bumped to v1.1.18 |

## Data Flow
1. User enters subscriber count → `ClientCountInput` debounces 300ms
2. `getClientTier(count)` computes tier (mirrors backend `ClientTierFromCount`)
3. `TierBadge` displays tier with color-coded gradient
4. User clicks "Select" on a tier card → spreads `localConfig` with tier-specific performance/cache values (same pattern as existing `applyProfile()`)
5. User clicks existing "Apply Tuning" button → `api.applyConfig(nodeId, state, reason)` sends payload including `client_count`

## Validation
- ✅ TypeScript: 0 errors (`npx tsc --noEmit`)
- ✅ Lint: 0 new warnings (`npx oxlint`)
- ✅ Build: success (`npm run build` → 549.60 kB JS, 54.60 kB CSS)
- ✅ Git: committed, tagged v1.1.18, pushed to origin/main

## Existing Features Preserved
- Hardware profile cards (SMALL, MEDIUM, LARGE, ISP) remain unchanged
- AI Auto-Apply Profile button unchanged
- Logrotate tab unchanged
- Resiliency tab unchanged
- All existing API contracts preserved
