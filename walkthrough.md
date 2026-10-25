# Walkthrough – Client‑Tier Tuning UI Design Plan

## Goal
Replace the legacy CPU/RAM‑centric tuning cards in **PerformanceView.tsx** with a modern, premium UI that recommends configuration profiles based on *client subscriber count* (Small, Medium, Large). The UI must be visually striking, responsive, and align with the backend auto‑tuning engine (`AutoTuneByClientCount`).

## High‑Level Architecture
1. **State Layer** – Extend the `DesiredState` model (in `web/src/types.ts`) to include `client_count` and `client_tier`.
2. **UI Layer** – New React components:
   - `ClientCountInput` – numeric input with live validation and a visual subscriber‑density slider.
   - `TierBadge` – dynamic badge showing the calculated tier (Small/Medium/Large) with colour‑coded gradients.
   - `TierProfileCard` – three cards (one per tier) presenting recommended settings, an *Apply* button, and a brief description.
   - `LegacyFallbackCard` – hidden but kept for backward compatibility; can be toggled via feature flag.
3. **Service Layer** – Wrapper around the existing API (`api.applyConfig`) that translates UI selections into the `DesiredState` payload, including the new `client_count` field.
4. **Styling Layer** – Add design tokens to `index.css` (or a dedicated `designTokens.css`) for:
   - Gradient backgrounds (`bg-gradient‑client‑small`, etc.)
   - Glass‑morphism card containers
   - Smooth micro‑animations on tier badge transitions and card hover.

## Detailed UI Component Blueprint
| Component | Props | Behaviour | Visual Highlights |
|-----------|-------|-----------|-------------------|
| **ClientCountInput** | `value: number`, `onChange: (n:number)=>void` | Debounced update (300 ms) → recompute tier via `getClientTier(count)`; shows helper tooltip with recommended range. | Input field with leading icon, subtle glow on focus, animated numeric stepper. |
| **TierBadge** | `tier: ClientTier` | Updates colour gradient on tier change, includes a small pulsing badge for “New” tier if count just crossed a boundary. | Rounded pill, gradient background, micro‑scale bounce animation. |
| **TierProfileCard** | `tier: ClientTier`, `profile: ProfileConfig` | Displays recommended `cores`, `ramMB`, `queueSize`, etc. *Apply* button triggers API call, shows loading spinner and success toast. | Glass‑morphic card, hover elevation, subtle shadow, animated entry (fade‑up). |
| **PerformanceView (updated)** | - | Integrates the above components; removes old hardware cards; adds a responsive grid (1‑column mobile, 3‑column desktop). | Full‑width background gradient, dark‑mode support, smooth scroll‑into‑view for cards. |

## Data Flow
1. User changes **client count** → `ClientCountInput` updates local state.
2. `useEffect` computes `client_tier` via `getClientTier(count)` (logic mirrors backend thresholds).
3. `TierBadge` reflects current tier instantly.
4. `TierProfileCard` for each tier pulls static profile data from a new constant file `clientTierProfiles.ts` (mirrors backend defaults).
5. Clicking **Apply** on a card sends `{ client_count, ...profile }` to `api.applyConfig`.
6. Backend validates and returns updated node config; UI shows success toast and refreshes summary.

## Styling & Aesthetics (per project standards)
- **Colour palette** – Cool blues with accent orange for “Large” tier, using HSL variables for easy theming.
- **Typography** – Google Font *Inter* (weights 400/600) loaded via `<link>` in `index.html`.
- **Micro‑animations** – `framer-motion` (already a dependency) for card entrance, badge pulse, button ripple.
- **Dark mode** – CSS media query `prefers-color-scheme` to switch gradients and text contrast.
- **Accessibility** – ARIA labels on inputs, focus‑visible outlines, sufficient colour contrast (>4.5:1).

## Implementation Steps (PLAN → IMPLEMENT → TEST → FIX → REGRESSION → REPORT → GATE)
1. **PLAN** – Finalise design tokens, create mockups (use `generate_image` if needed).
2. **IMPLEMENT**
   - Update `types.ts` (already done) – ensure export of `ClientTier`.
   - Add `clientTierProfiles.ts` with static profile objects.
   - Create new component files under `web/src/components/`.
   - Refactor `PerformanceView.tsx` to import and render new components, remove old hardware cards.
   - Add CSS variables and card styles in `web/src/index.css`.
3. **TEST** – Write unit tests for `getClientTier` utility, integration test for API payload using Jest + React Testing Library.
4. **FIX** – Address any failing tests, lint errors, and visual regressions.
5. **REGRESSION** – Run full UI test suite, ensure other views unaffected.
6. **REPORT** – Summarise changes in a PR description, update `CHANGELOG.md` with version `1.1.17`.
7. **GATE** – Review by senior dev, merge to `main` after CI passes.
8. **DEPLOY** – Deploy to staging, perform smoke test, then roll out to production.

## Rollout & Versioning
- Bump project version to **v1.1.17** in `package.json` and backend `go.mod` tag.
- Tag a Git release `v1.1.17‑client‑tuning`.
- Include migration note in this walkthrough.

## Risks & Mitigations
| Risk | Impact | Mitigation |
|------|--------|------------|
| Incorrect tier calculation due to mismatched thresholds | Mis‑configuration of nodes | Keep thresholds in a shared constant (`src/constants/clientTier.ts`) and unit‑test against backend values. |
| UI performance lag on low‑end browsers | Poor UX | Lazy‑load heavy animation libraries, limit re‑renders with `React.memo`. |
| Regression of existing performance view features | Functional breakage | Preserve legacy cards behind a feature flag (`ENABLE_LEGACY_TUNING`). |

---
*Prepared by Antigravity agent – design plan ready for implementation.*
