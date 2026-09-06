# Changelog

## v1.0.1 Hotfix

- Fixed the three mini-selects in Servant Box cards rendering as blank under Pico CSS by overriding the framework's select right-padding and using compact labels.
- Forced GitHub/Chaldea git probes to HTTP/1.1 to avoid the observed HTTP/2 reset path on some Windows networks.
- If Chaldea Data cannot be refreshed but a local Chaldea cache exists, Data Sync now continues from that cache instead of aborting immediately.
- Data Sync failures keep full diagnostics in logs/devtools but show a concise message in the UI and explicitly preserve already-loaded local data.

## v9 Upstream Rebase / GitHub Candidate

- Tweaked the optimization-mode hint spacing for clearer visual separation from the selector.
- Rebased the local fork's solver semantics onto the 2026-09-06 GPL-3.0 upstream changes.
- Added active Bond 15 -> 16 handling: the servant earns bond and still provides party +25%.
- Added selected-party `event_party_bonuses` handling and per-servant result accounting.
- Fixed explicit include vs exclude conflicts for servants and CEs.
- Removed the legacy cost-derived minimum-CE pruning heuristic.
- Updated reviewed bond CE mappings, including CE 9409490; CE 9408800 is no longer treated as JP-only.
- Preserved Local Box features: Grand 8 bonus slots, owned Box/CE restrictions, driver/passenger planner, fixed members, Stream/toplogin import, Chaldea CSV, exact bond breakdown, Data Sync.
- Preserved Local-only architecture: no login/register/JWT/SQLite user DB; loopback-only backend.

## v8 Local-only

- Removed upstream account/auth/cloud state features and restricted the backend to loopback.
