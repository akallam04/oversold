# counter-two-servers-redis-locking

- Scenario file: `counter-two-servers-redis-locking.yaml`, sha256 `27353159e70fb272a931c61cd6079cd26fd6b9ba1d0cb062b88f08dff36f1092`
- Medusa version in the file: 2.21.2. Version used: 2.22.0
- Oversold commit that ran it: `73e03abeee95629300f422b57b9750585cbc18de` (clean)
- Verdicts computed by Oversold commit: `73e03abeee95629300f422b57b9750585cbc18de` (clean)
- Server `medusa`: Medusa 2.22.0, image `sha256:0c78357f67a55dfb7ba59325bb9bebf985553495a3d7b3cf2b8b4936ef0e2814`
- Server `medusa-2`: Medusa 2.22.0, image `sha256:0c78357f67a55dfb7ba59325bb9bebf985553495a3d7b3cf2b8b4936ef0e2814`
- Postgres 15.19
- Locking: redis
- Redis 7.4.11
- Scripted Redis lock calls during the race: 59 to 67 per run, recorded in 30 runs
- Runs: 30 of 30 recorded
- Exit code: 0

## Checks

| Check | Version | Fail | Pass | Inconclusive |
|---|---|---|---|---|
| no_oversell | 1 | 0 of 30 | 30 of 30 | 0 of 30 |
| reserved_matches_reservations | 1 | 0 of 30 | 30 of 30 | 0 of 30 |
| one_order_per_cart | 1 | 0 of 30 | 30 of 30 | 0 of 30 |
| no_orphaned_orders | 1 | 0 of 30 | 30 of 30 | 0 of 30 |

## Runs

| Run | no_oversell | reserved_matches_reservations | one_order_per_cart | no_orphaned_orders | Lock calls | History |
|---|---|---|---|---|---|---|
| 1 | pass | pass | pass | pass | 64 | [run-001.json.gz](run-001.json.gz) |
| 2 | pass | pass | pass | pass | 61 | [run-002.json.gz](run-002.json.gz) |
| 3 | pass | pass | pass | pass | 63 | [run-003.json.gz](run-003.json.gz) |
| 4 | pass | pass | pass | pass | 61 | [run-004.json.gz](run-004.json.gz) |
| 5 | pass | pass | pass | pass | 66 | [run-005.json.gz](run-005.json.gz) |
| 6 | pass | pass | pass | pass | 63 | [run-006.json.gz](run-006.json.gz) |
| 7 | pass | pass | pass | pass | 59 | [run-007.json.gz](run-007.json.gz) |
| 8 | pass | pass | pass | pass | 65 | [run-008.json.gz](run-008.json.gz) |
| 9 | pass | pass | pass | pass | 65 | [run-009.json.gz](run-009.json.gz) |
| 10 | pass | pass | pass | pass | 64 | [run-010.json.gz](run-010.json.gz) |
| 11 | pass | pass | pass | pass | 62 | [run-011.json.gz](run-011.json.gz) |
| 12 | pass | pass | pass | pass | 65 | [run-012.json.gz](run-012.json.gz) |
| 13 | pass | pass | pass | pass | 62 | [run-013.json.gz](run-013.json.gz) |
| 14 | pass | pass | pass | pass | 63 | [run-014.json.gz](run-014.json.gz) |
| 15 | pass | pass | pass | pass | 63 | [run-015.json.gz](run-015.json.gz) |
| 16 | pass | pass | pass | pass | 63 | [run-016.json.gz](run-016.json.gz) |
| 17 | pass | pass | pass | pass | 67 | [run-017.json.gz](run-017.json.gz) |
| 18 | pass | pass | pass | pass | 63 | [run-018.json.gz](run-018.json.gz) |
| 19 | pass | pass | pass | pass | 63 | [run-019.json.gz](run-019.json.gz) |
| 20 | pass | pass | pass | pass | 64 | [run-020.json.gz](run-020.json.gz) |
| 21 | pass | pass | pass | pass | 64 | [run-021.json.gz](run-021.json.gz) |
| 22 | pass | pass | pass | pass | 62 | [run-022.json.gz](run-022.json.gz) |
| 23 | pass | pass | pass | pass | 62 | [run-023.json.gz](run-023.json.gz) |
| 24 | pass | pass | pass | pass | 63 | [run-024.json.gz](run-024.json.gz) |
| 25 | pass | pass | pass | pass | 62 | [run-025.json.gz](run-025.json.gz) |
| 26 | pass | pass | pass | pass | 65 | [run-026.json.gz](run-026.json.gz) |
| 27 | pass | pass | pass | pass | 61 | [run-027.json.gz](run-027.json.gz) |
| 28 | pass | pass | pass | pass | 61 | [run-028.json.gz](run-028.json.gz) |
| 29 | pass | pass | pass | pass | 63 | [run-029.json.gz](run-029.json.gz) |
| 30 | pass | pass | pass | pass | 61 | [run-030.json.gz](run-030.json.gz) |
