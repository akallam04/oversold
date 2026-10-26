# normal-checkout-two-servers-redis-locking

- Scenario file: `normal-checkout-two-servers-redis-locking.yaml`, sha256 `1bae0e8eea40090975a7f698dc0791b7cb142bcfa864d68c00410a125e5cbe2a`
- Medusa version in the file: 2.21.2. Version used: 2.21.2
- Oversold commit that ran it: `73e03abeee95629300f422b57b9750585cbc18de` (clean)
- Verdicts computed by Oversold commit: `73e03abeee95629300f422b57b9750585cbc18de` (clean)
- Server `medusa`: Medusa 2.21.2, image `sha256:56be8697b2ac544854583fd3dc79d02f6e48990ea1d51076281c8baf67da63cb`
- Server `medusa-2`: Medusa 2.21.2, image `sha256:56be8697b2ac544854583fd3dc79d02f6e48990ea1d51076281c8baf67da63cb`
- Postgres 15.19
- Locking: redis
- Redis 7.4.11
- Scripted Redis lock calls during the race: 4 to 4 per run, recorded in 30 runs
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
| 1 | pass | pass | pass | pass | 4 | [run-001.json.gz](run-001.json.gz) |
| 2 | pass | pass | pass | pass | 4 | [run-002.json.gz](run-002.json.gz) |
| 3 | pass | pass | pass | pass | 4 | [run-003.json.gz](run-003.json.gz) |
| 4 | pass | pass | pass | pass | 4 | [run-004.json.gz](run-004.json.gz) |
| 5 | pass | pass | pass | pass | 4 | [run-005.json.gz](run-005.json.gz) |
| 6 | pass | pass | pass | pass | 4 | [run-006.json.gz](run-006.json.gz) |
| 7 | pass | pass | pass | pass | 4 | [run-007.json.gz](run-007.json.gz) |
| 8 | pass | pass | pass | pass | 4 | [run-008.json.gz](run-008.json.gz) |
| 9 | pass | pass | pass | pass | 4 | [run-009.json.gz](run-009.json.gz) |
| 10 | pass | pass | pass | pass | 4 | [run-010.json.gz](run-010.json.gz) |
| 11 | pass | pass | pass | pass | 4 | [run-011.json.gz](run-011.json.gz) |
| 12 | pass | pass | pass | pass | 4 | [run-012.json.gz](run-012.json.gz) |
| 13 | pass | pass | pass | pass | 4 | [run-013.json.gz](run-013.json.gz) |
| 14 | pass | pass | pass | pass | 4 | [run-014.json.gz](run-014.json.gz) |
| 15 | pass | pass | pass | pass | 4 | [run-015.json.gz](run-015.json.gz) |
| 16 | pass | pass | pass | pass | 4 | [run-016.json.gz](run-016.json.gz) |
| 17 | pass | pass | pass | pass | 4 | [run-017.json.gz](run-017.json.gz) |
| 18 | pass | pass | pass | pass | 4 | [run-018.json.gz](run-018.json.gz) |
| 19 | pass | pass | pass | pass | 4 | [run-019.json.gz](run-019.json.gz) |
| 20 | pass | pass | pass | pass | 4 | [run-020.json.gz](run-020.json.gz) |
| 21 | pass | pass | pass | pass | 4 | [run-021.json.gz](run-021.json.gz) |
| 22 | pass | pass | pass | pass | 4 | [run-022.json.gz](run-022.json.gz) |
| 23 | pass | pass | pass | pass | 4 | [run-023.json.gz](run-023.json.gz) |
| 24 | pass | pass | pass | pass | 4 | [run-024.json.gz](run-024.json.gz) |
| 25 | pass | pass | pass | pass | 4 | [run-025.json.gz](run-025.json.gz) |
| 26 | pass | pass | pass | pass | 4 | [run-026.json.gz](run-026.json.gz) |
| 27 | pass | pass | pass | pass | 4 | [run-027.json.gz](run-027.json.gz) |
| 28 | pass | pass | pass | pass | 4 | [run-028.json.gz](run-028.json.gz) |
| 29 | pass | pass | pass | pass | 4 | [run-029.json.gz](run-029.json.gz) |
| 30 | pass | pass | pass | pass | 4 | [run-030.json.gz](run-030.json.gz) |
