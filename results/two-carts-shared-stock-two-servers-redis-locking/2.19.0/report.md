# two-carts-shared-stock-two-servers-redis-locking

- Scenario file: `two-carts-shared-stock-two-servers-redis-locking.yaml`, sha256 `701f6c10f145837200c57eccaecf7fe3b0f029d1b7f4424bc0296840cc844638`
- Medusa version in the file: 2.21.2. Version used: 2.19.0
- Oversold commit that ran it: `73e03abeee95629300f422b57b9750585cbc18de` (clean)
- Verdicts computed by Oversold commit: `73e03abeee95629300f422b57b9750585cbc18de` (clean)
- Server `medusa`: Medusa 2.19.0, image `sha256:6a21dbc25ef2a92d21a72db65f885855c6309a5861e9f5b7d3487f723d87d7c6`
- Server `medusa-2`: Medusa 2.19.0, image `sha256:6a21dbc25ef2a92d21a72db65f885855c6309a5861e9f5b7d3487f723d87d7c6`
- Postgres 15.19
- Locking: redis
- Redis 7.4.11
- Scripted Redis lock calls during the race: 9 to 9 per run, recorded in 30 runs
- Runs: 30 of 30 recorded
- Exit code: 1

## Checks

| Check | Version | Fail | Pass | Inconclusive |
|---|---|---|---|---|
| no_oversell | 1 | 29 of 30 | 1 of 30 | 0 of 30 |
| reserved_matches_reservations | 1 | 0 of 30 | 30 of 30 | 0 of 30 |
| one_order_per_cart | 1 | 0 of 30 | 30 of 30 | 0 of 30 |
| no_orphaned_orders | 1 | 0 of 30 | 30 of 30 | 0 of 30 |

## Runs

| Run | no_oversell | reserved_matches_reservations | one_order_per_cart | no_orphaned_orders | Lock calls | History |
|---|---|---|---|---|---|---|
| 1 | fail | pass | pass | pass | 9 | [run-001.json.gz](run-001.json.gz) |
| 2 | pass | pass | pass | pass | 9 | [run-002.json.gz](run-002.json.gz) |
| 3 | fail | pass | pass | pass | 9 | [run-003.json.gz](run-003.json.gz) |
| 4 | fail | pass | pass | pass | 9 | [run-004.json.gz](run-004.json.gz) |
| 5 | fail | pass | pass | pass | 9 | [run-005.json.gz](run-005.json.gz) |
| 6 | fail | pass | pass | pass | 9 | [run-006.json.gz](run-006.json.gz) |
| 7 | fail | pass | pass | pass | 9 | [run-007.json.gz](run-007.json.gz) |
| 8 | fail | pass | pass | pass | 9 | [run-008.json.gz](run-008.json.gz) |
| 9 | fail | pass | pass | pass | 9 | [run-009.json.gz](run-009.json.gz) |
| 10 | fail | pass | pass | pass | 9 | [run-010.json.gz](run-010.json.gz) |
| 11 | fail | pass | pass | pass | 9 | [run-011.json.gz](run-011.json.gz) |
| 12 | fail | pass | pass | pass | 9 | [run-012.json.gz](run-012.json.gz) |
| 13 | fail | pass | pass | pass | 9 | [run-013.json.gz](run-013.json.gz) |
| 14 | fail | pass | pass | pass | 9 | [run-014.json.gz](run-014.json.gz) |
| 15 | fail | pass | pass | pass | 9 | [run-015.json.gz](run-015.json.gz) |
| 16 | fail | pass | pass | pass | 9 | [run-016.json.gz](run-016.json.gz) |
| 17 | fail | pass | pass | pass | 9 | [run-017.json.gz](run-017.json.gz) |
| 18 | fail | pass | pass | pass | 9 | [run-018.json.gz](run-018.json.gz) |
| 19 | fail | pass | pass | pass | 9 | [run-019.json.gz](run-019.json.gz) |
| 20 | fail | pass | pass | pass | 9 | [run-020.json.gz](run-020.json.gz) |
| 21 | fail | pass | pass | pass | 9 | [run-021.json.gz](run-021.json.gz) |
| 22 | fail | pass | pass | pass | 9 | [run-022.json.gz](run-022.json.gz) |
| 23 | fail | pass | pass | pass | 9 | [run-023.json.gz](run-023.json.gz) |
| 24 | fail | pass | pass | pass | 9 | [run-024.json.gz](run-024.json.gz) |
| 25 | fail | pass | pass | pass | 9 | [run-025.json.gz](run-025.json.gz) |
| 26 | fail | pass | pass | pass | 9 | [run-026.json.gz](run-026.json.gz) |
| 27 | fail | pass | pass | pass | 9 | [run-027.json.gz](run-027.json.gz) |
| 28 | fail | pass | pass | pass | 9 | [run-028.json.gz](run-028.json.gz) |
| 29 | fail | pass | pass | pass | 9 | [run-029.json.gz](run-029.json.gz) |
| 30 | fail | pass | pass | pass | 9 | [run-030.json.gz](run-030.json.gz) |

## Failed and inconclusive verdicts

- Run 1, no_oversell fail: WIDGET live reservations 12 exceed stocked 10; WIDGET orders need 12 units but 10 were stocked at the start of the race
- Run 3, no_oversell fail: WIDGET live reservations 12 exceed stocked 10; WIDGET orders need 12 units but 10 were stocked at the start of the race
- Run 4, no_oversell fail: WIDGET live reservations 12 exceed stocked 10; WIDGET orders need 12 units but 10 were stocked at the start of the race
- Run 5, no_oversell fail: WIDGET live reservations 12 exceed stocked 10; WIDGET orders need 12 units but 10 were stocked at the start of the race
- Run 6, no_oversell fail: WIDGET live reservations 12 exceed stocked 10; WIDGET orders need 12 units but 10 were stocked at the start of the race
- Run 7, no_oversell fail: WIDGET live reservations 12 exceed stocked 10; WIDGET orders need 12 units but 10 were stocked at the start of the race
- Run 8, no_oversell fail: WIDGET live reservations 12 exceed stocked 10; WIDGET orders need 12 units but 10 were stocked at the start of the race
- Run 9, no_oversell fail: WIDGET live reservations 12 exceed stocked 10; WIDGET orders need 12 units but 10 were stocked at the start of the race
- Run 10, no_oversell fail: WIDGET live reservations 12 exceed stocked 10; WIDGET orders need 12 units but 10 were stocked at the start of the race
- Run 11, no_oversell fail: WIDGET live reservations 12 exceed stocked 10; WIDGET orders need 12 units but 10 were stocked at the start of the race
- Run 12, no_oversell fail: WIDGET live reservations 12 exceed stocked 10; WIDGET orders need 12 units but 10 were stocked at the start of the race
- Run 13, no_oversell fail: WIDGET live reservations 12 exceed stocked 10; WIDGET orders need 12 units but 10 were stocked at the start of the race
- Run 14, no_oversell fail: WIDGET live reservations 12 exceed stocked 10; WIDGET orders need 12 units but 10 were stocked at the start of the race
- Run 15, no_oversell fail: WIDGET live reservations 12 exceed stocked 10; WIDGET orders need 12 units but 10 were stocked at the start of the race
- Run 16, no_oversell fail: WIDGET live reservations 12 exceed stocked 10; WIDGET orders need 12 units but 10 were stocked at the start of the race
- Run 17, no_oversell fail: WIDGET live reservations 12 exceed stocked 10; WIDGET orders need 12 units but 10 were stocked at the start of the race
- Run 18, no_oversell fail: WIDGET live reservations 12 exceed stocked 10; WIDGET orders need 12 units but 10 were stocked at the start of the race
- Run 19, no_oversell fail: WIDGET live reservations 12 exceed stocked 10; WIDGET orders need 12 units but 10 were stocked at the start of the race
- Run 20, no_oversell fail: WIDGET live reservations 12 exceed stocked 10; WIDGET orders need 12 units but 10 were stocked at the start of the race
- Run 21, no_oversell fail: WIDGET live reservations 12 exceed stocked 10; WIDGET orders need 12 units but 10 were stocked at the start of the race
- Run 22, no_oversell fail: WIDGET live reservations 12 exceed stocked 10; WIDGET orders need 12 units but 10 were stocked at the start of the race
- Run 23, no_oversell fail: WIDGET live reservations 12 exceed stocked 10; WIDGET orders need 12 units but 10 were stocked at the start of the race
- Run 24, no_oversell fail: WIDGET live reservations 12 exceed stocked 10; WIDGET orders need 12 units but 10 were stocked at the start of the race
- Run 25, no_oversell fail: WIDGET live reservations 12 exceed stocked 10; WIDGET orders need 12 units but 10 were stocked at the start of the race
- Run 26, no_oversell fail: WIDGET live reservations 12 exceed stocked 10; WIDGET orders need 12 units but 10 were stocked at the start of the race
- Run 27, no_oversell fail: WIDGET live reservations 12 exceed stocked 10; WIDGET orders need 12 units but 10 were stocked at the start of the race
- Run 28, no_oversell fail: WIDGET live reservations 12 exceed stocked 10; WIDGET orders need 12 units but 10 were stocked at the start of the race
- Run 29, no_oversell fail: WIDGET live reservations 12 exceed stocked 10; WIDGET orders need 12 units but 10 were stocked at the start of the race
- Run 30, no_oversell fail: WIDGET live reservations 12 exceed stocked 10; WIDGET orders need 12 units but 10 were stocked at the start of the race
