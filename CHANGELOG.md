# Changelog

## v1.0.2 Windows Portable

- Added a GitHub Actions Windows portable build using Go 1.25.1 from `backend/go.mod`.
- The release package includes a precompiled `fgo-calc-local.exe`, static assets, current game data, update scripts, license/docs, and a one-click launcher.
- Added a portable smoke test that starts the packaged backend and verifies `http://127.0.0.1:30006/test` before publishing the artifact.
- Normal Windows use no longer requires Go or a local compile step; users only need to download, extract, and run `Start FGO Calc.bat`.
- Python 3.10+ and Git remain optional dependencies used only by Data Sync / game-data updates.
- On clean Windows systems without Python, the automatic Data Sync status check now falls back to the bundled dataset as a normal offline state instead of showing a failure; Windows exit status 9009 is treated as an unavailable optional sync environment.
- Added `SHA256SUMS.txt` for the generated Windows ZIP.

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
