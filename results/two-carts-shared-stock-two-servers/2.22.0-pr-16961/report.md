# two-carts-shared-stock-two-servers

- Scenario file: `two-carts-shared-stock-two-servers.yaml`, sha256 `33d016ea9369fd8c16d093a42b0b840e9659851c1fdd7e049894d6398b9bc0cd`
- Medusa version in the file: 2.21.2. Version used: 2.22.0
- Oversold commit that ran it: `012083b1feeee16e290b8050f53e76b51f53bf72` (clean)
- Verdicts computed by Oversold commit: `012083b1feeee16e290b8050f53e76b51f53bf72` (clean)
- Variant `pr-16961`: Medusa 2.22.0 with the source change of https://github.com/medusajs/medusa/pull/16961 by pqa-labs (closed, not merged), head commit `5ac8ae437ee0b7771a945b5b30f265b76f214172`
  - Replaces `@medusajs/inventory/dist/services/inventory-module.js`, sha256 `8e150f6b9de7bc64f431e91a803716646a782379d3cc993a31f301056d5ac919` (was `6d2643ef94e7ffe20fb2b4a35f8f1de503ad4429c85c64cc4262ceae2a776c58`)
  - Replaces `@medusajs/core-flows/dist/cart/steps/confirm-inventory.js`, sha256 `2af8432c719836364a143d9fe7f4ee6f9aba17062d555e2fdace47315c11c569` (was `01a7cf229b6854a7840207c6ba62234bbdedc651adfd50276536e06d6e46134d`)
- Server `medusa`: Medusa 2.22.0, image `sha256:48986d179a7b342c15456b7bbe655d539d4dd84ee14085111fecaebf900e3cce`
  - Image inputs sha256 `d7306668cba9594398c0dc9e3289bbbadeabad6ad7c20c9292104713ebd68257`
- Server `medusa-2`: Medusa 2.22.0, image `sha256:48986d179a7b342c15456b7bbe655d539d4dd84ee14085111fecaebf900e3cce`
  - Image inputs sha256 `d7306668cba9594398c0dc9e3289bbbadeabad6ad7c20c9292104713ebd68257`
- Postgres 15.19
- Locking: in-memory
- Runs: 30 of 30 recorded
- Exit code: 1

## Checks

| Check | Version | Fail | Pass | Inconclusive |
|---|---|---|---|---|
| no_oversell | 1 | 29 of 30 | 1 of 30 | 0 of 30 |
| reserved_matches_reservations | 1 | 29 of 30 | 1 of 30 | 0 of 30 |
| one_order_per_cart | 1 | 0 of 30 | 30 of 30 | 0 of 30 |
| no_orphaned_orders | 1 | 0 of 30 | 30 of 30 | 0 of 30 |

## Runs

| Run | no_oversell | reserved_matches_reservations | one_order_per_cart | no_orphaned_orders | History |
|---|---|---|---|---|---|
| 1 | fail | fail | pass | pass | [run-001.json.gz](run-001.json.gz) |
| 2 | fail | fail | pass | pass | [run-002.json.gz](run-002.json.gz) |
| 3 | fail | fail | pass | pass | [run-003.json.gz](run-003.json.gz) |
| 4 | fail | fail | pass | pass | [run-004.json.gz](run-004.json.gz) |
| 5 | fail | fail | pass | pass | [run-005.json.gz](run-005.json.gz) |
| 6 | fail | fail | pass | pass | [run-006.json.gz](run-006.json.gz) |
| 7 | fail | fail | pass | pass | [run-007.json.gz](run-007.json.gz) |
| 8 | fail | fail | pass | pass | [run-008.json.gz](run-008.json.gz) |
| 9 | fail | fail | pass | pass | [run-009.json.gz](run-009.json.gz) |
| 10 | fail | fail | pass | pass | [run-010.json.gz](run-010.json.gz) |
| 11 | fail | fail | pass | pass | [run-011.json.gz](run-011.json.gz) |
| 12 | fail | fail | pass | pass | [run-012.json.gz](run-012.json.gz) |
| 13 | fail | fail | pass | pass | [run-013.json.gz](run-013.json.gz) |
| 14 | fail | fail | pass | pass | [run-014.json.gz](run-014.json.gz) |
| 15 | fail | fail | pass | pass | [run-015.json.gz](run-015.json.gz) |
| 16 | pass | pass | pass | pass | [run-016.json.gz](run-016.json.gz) |
| 17 | fail | fail | pass | pass | [run-017.json.gz](run-017.json.gz) |
| 18 | fail | fail | pass | pass | [run-018.json.gz](run-018.json.gz) |
| 19 | fail | fail | pass | pass | [run-019.json.gz](run-019.json.gz) |
| 20 | fail | fail | pass | pass | [run-020.json.gz](run-020.json.gz) |
| 21 | fail | fail | pass | pass | [run-021.json.gz](run-021.json.gz) |
| 22 | fail | fail | pass | pass | [run-022.json.gz](run-022.json.gz) |
| 23 | fail | fail | pass | pass | [run-023.json.gz](run-023.json.gz) |
| 24 | fail | fail | pass | pass | [run-024.json.gz](run-024.json.gz) |
| 25 | fail | fail | pass | pass | [run-025.json.gz](run-025.json.gz) |
| 26 | fail | fail | pass | pass | [run-026.json.gz](run-026.json.gz) |
| 27 | fail | fail | pass | pass | [run-027.json.gz](run-027.json.gz) |
| 28 | fail | fail | pass | pass | [run-028.json.gz](run-028.json.gz) |
| 29 | fail | fail | pass | pass | [run-029.json.gz](run-029.json.gz) |
| 30 | fail | fail | pass | pass | [run-030.json.gz](run-030.json.gz) |

## Failed and inconclusive verdicts

- Run 1, no_oversell fail: WIDGET live reservations 12 exceed stocked 10; WIDGET orders need 12 units but 10 were stocked at the start of the race
- Run 1, reserved_matches_reservations fail: WIDGET reserved_quantity 7 but live reservations sum to 12
- Run 2, no_oversell fail: WIDGET live reservations 12 exceed stocked 10; WIDGET orders need 12 units but 10 were stocked at the start of the race
- Run 2, reserved_matches_reservations fail: WIDGET reserved_quantity 7 but live reservations sum to 12
- Run 3, no_oversell fail: WIDGET live reservations 12 exceed stocked 10; WIDGET orders need 12 units but 10 were stocked at the start of the race
- Run 3, reserved_matches_reservations fail: WIDGET reserved_quantity 7 but live reservations sum to 12
- Run 4, no_oversell fail: WIDGET live reservations 12 exceed stocked 10; WIDGET orders need 12 units but 10 were stocked at the start of the race
- Run 4, reserved_matches_reservations fail: WIDGET reserved_quantity 7 but live reservations sum to 12
- Run 5, no_oversell fail: WIDGET live reservations 12 exceed stocked 10; WIDGET orders need 12 units but 10 were stocked at the start of the race
- Run 5, reserved_matches_reservations fail: WIDGET reserved_quantity 7 but live reservations sum to 12
- Run 6, no_oversell fail: WIDGET live reservations 12 exceed stocked 10; WIDGET orders need 12 units but 10 were stocked at the start of the race
- Run 6, reserved_matches_reservations fail: WIDGET reserved_quantity 7 but live reservations sum to 12
- Run 7, no_oversell fail: WIDGET live reservations 12 exceed stocked 10; WIDGET orders need 12 units but 10 were stocked at the start of the race
- Run 7, reserved_matches_reservations fail: WIDGET reserved_quantity 7 but live reservations sum to 12
- Run 8, no_oversell fail: WIDGET live reservations 12 exceed stocked 10; WIDGET orders need 12 units but 10 were stocked at the start of the race
- Run 8, reserved_matches_reservations fail: WIDGET reserved_quantity 7 but live reservations sum to 12
- Run 9, no_oversell fail: WIDGET live reservations 12 exceed stocked 10; WIDGET orders need 12 units but 10 were stocked at the start of the race
- Run 9, reserved_matches_reservations fail: WIDGET reserved_quantity 5 but live reservations sum to 12
- Run 10, no_oversell fail: WIDGET live reservations 12 exceed stocked 10; WIDGET orders need 12 units but 10 were stocked at the start of the race
- Run 10, reserved_matches_reservations fail: WIDGET reserved_quantity 7 but live reservations sum to 12
- Run 11, no_oversell fail: WIDGET live reservations 12 exceed stocked 10; WIDGET orders need 12 units but 10 were stocked at the start of the race
- Run 11, reserved_matches_reservations fail: WIDGET reserved_quantity 7 but live reservations sum to 12
- Run 12, no_oversell fail: WIDGET live reservations 12 exceed stocked 10; WIDGET orders need 12 units but 10 were stocked at the start of the race
- Run 12, reserved_matches_reservations fail: WIDGET reserved_quantity 7 but live reservations sum to 12
- Run 13, no_oversell fail: WIDGET live reservations 12 exceed stocked 10; WIDGET orders need 12 units but 10 were stocked at the start of the race
- Run 13, reserved_matches_reservations fail: WIDGET reserved_quantity 7 but live reservations sum to 12
- Run 14, no_oversell fail: WIDGET live reservations 12 exceed stocked 10; WIDGET orders need 12 units but 10 were stocked at the start of the race
- Run 14, reserved_matches_reservations fail: WIDGET reserved_quantity 7 but live reservations sum to 12
- Run 15, no_oversell fail: WIDGET live reservations 12 exceed stocked 10; WIDGET orders need 12 units but 10 were stocked at the start of the race
- Run 15, reserved_matches_reservations fail: WIDGET reserved_quantity 7 but live reservations sum to 12
- Run 17, no_oversell fail: WIDGET live reservations 12 exceed stocked 10; WIDGET orders need 12 units but 10 were stocked at the start of the race
- Run 17, reserved_matches_reservations fail: WIDGET reserved_quantity 7 but live reservations sum to 12
- Run 18, no_oversell fail: WIDGET live reservations 12 exceed stocked 10; WIDGET orders need 12 units but 10 were stocked at the start of the race
- Run 18, reserved_matches_reservations fail: WIDGET reserved_quantity 7 but live reservations sum to 12
- Run 19, no_oversell fail: WIDGET live reservations 12 exceed stocked 10; WIDGET orders need 12 units but 10 were stocked at the start of the race
- Run 19, reserved_matches_reservations fail: WIDGET reserved_quantity 7 but live reservations sum to 12
- Run 20, no_oversell fail: WIDGET live reservations 12 exceed stocked 10; WIDGET orders need 12 units but 10 were stocked at the start of the race
- Run 20, reserved_matches_reservations fail: WIDGET reserved_quantity 7 but live reservations sum to 12
- Run 21, no_oversell fail: WIDGET live reservations 12 exceed stocked 10; WIDGET orders need 12 units but 10 were stocked at the start of the race
- Run 21, reserved_matches_reservations fail: WIDGET reserved_quantity 7 but live reservations sum to 12
- Run 22, no_oversell fail: WIDGET live reservations 12 exceed stocked 10; WIDGET orders need 12 units but 10 were stocked at the start of the race
- Run 22, reserved_matches_reservations fail: WIDGET reserved_quantity 5 but live reservations sum to 12
- Run 23, no_oversell fail: WIDGET live reservations 12 exceed stocked 10; WIDGET orders need 12 units but 10 were stocked at the start of the race
- Run 23, reserved_matches_reservations fail: WIDGET reserved_quantity 5 but live reservations sum to 12
- Run 24, no_oversell fail: WIDGET live reservations 12 exceed stocked 10; WIDGET orders need 12 units but 10 were stocked at the start of the race
- Run 24, reserved_matches_reservations fail: WIDGET reserved_quantity 7 but live reservations sum to 12
- Run 25, no_oversell fail: WIDGET live reservations 12 exceed stocked 10; WIDGET orders need 12 units but 10 were stocked at the start of the race
- Run 25, reserved_matches_reservations fail: WIDGET reserved_quantity 7 but live reservations sum to 12
- Run 26, no_oversell fail: WIDGET live reservations 12 exceed stocked 10; WIDGET orders need 12 units but 10 were stocked at the start of the race
- Run 26, reserved_matches_reservations fail: WIDGET reserved_quantity 7 but live reservations sum to 12
- Run 27, no_oversell fail: WIDGET live reservations 12 exceed stocked 10; WIDGET orders need 12 units but 10 were stocked at the start of the race
- Run 27, reserved_matches_reservations fail: WIDGET reserved_quantity 7 but live reservations sum to 12
- Run 28, no_oversell fail: WIDGET live reservations 12 exceed stocked 10; WIDGET orders need 12 units but 10 were stocked at the start of the race
- Run 28, reserved_matches_reservations fail: WIDGET reserved_quantity 5 but live reservations sum to 12
- Run 29, no_oversell fail: WIDGET live reservations 12 exceed stocked 10; WIDGET orders need 12 units but 10 were stocked at the start of the race
- Run 29, reserved_matches_reservations fail: WIDGET reserved_quantity 5 but live reservations sum to 12
- Run 30, no_oversell fail: WIDGET live reservations 12 exceed stocked 10; WIDGET orders need 12 units but 10 were stocked at the start of the race
- Run 30, reserved_matches_reservations fail: WIDGET reserved_quantity 7 but live reservations sum to 12
