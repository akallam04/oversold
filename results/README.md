# Results

Measured runs of every scenario in [scenarios/medusa/](../scenarios/medusa/), 30 runs each, on Medusa 2.22.0 (released Oct 21, 2026), 2.21.2 (released Sep 28, 2026) and 2.19.0 (released Aug 13, 2026). Each folder holds the scenario file that was run, `report.json`, `report.md` and every run's history, gzipped. The checks are defined in [docs/checks.md](../docs/checks.md), and the history format in [docs/history-format.md](../docs/history-format.md).

Each check cell counts the runs where that check failed, out of the runs recorded. The folders named `2.22.0-pr-16961` hold runs of the pr-16961 build variant, described in [its own section](#build-variant-pr-16961) below. They are not rows of the main table. The exit code is the one `oversold run` gave for that run set: 0 when every check passed in every run, 1 when any check failed in any run.

On 2.22.0, the same checks failed in the same scenarios as on 2.21.2. The fail counts differ only for two-carts-shared-stock (15 of 30 on 2.22.0, 14 of 30 on 2.21.2) and last-unit-two-servers (29 of 30 on 2.22.0, 28 of 30 on 2.21.2).

single-cart-over-demand-stock-drop builds the same cart as single-cart-over-demand (OVS-PIECE 4 and OVS-TWOPACK 2, 8 WIDGET units) while 10 WIDGET are in stock, then sets stock to 6 with `stock_before_race` and completes the cart once. It failed no_oversell in 30 of 30 runs on each of the three versions, like single-cart-over-demand, with one order holding 8 units against 6 in stock in every run.

The Locking column says which Locking Module provider the Medusa servers used. The five scenarios ending in `-redis-locking` are copies of the five two-server scenarios with `target.locking: redis`: both servers use Medusa's Redis locking provider, and every other Medusa module stays in-memory. Every other row used Medusa's default in-memory provider. With Redis locking, on all three versions:

- last-unit-two-servers-redis-locking and counter-two-servers-redis-locking failed no check in any of their 90 runs. With in-memory locking, last-unit-two-servers failed no_oversell and reserved_matches_reservations in 87 of 90 runs, and counter-two-servers failed reserved_matches_reservations in 90 of 90.
- two-carts-shared-stock-two-servers-redis-locking failed no_oversell in 78 of 90 runs and reserved_matches_reservations in none. With in-memory locking, two-carts-shared-stock-two-servers failed them in 90 and 89 of 90. Every run that failed no_oversell is one where cart a's complete returned before cart b's and both placed an order. In every run where cart b's complete returned first, cart a's complete was rejected with "Not enough stock available". two-carts-a-then-b, which completes cart a and then cart b one after the other on one server, fails no_oversell in 30 of 30 runs on each version.
- The two controls failed no check, as with in-memory locking.
- No run had a harness error, an inconclusive check or a lock timeout, and the only rejected completes said "Not enough stock available".

| Scenario | Medusa | Locking | Targets | no_oversell | reserved_matches_reservations | one_order_per_cart | no_orphaned_orders | Inconclusive runs | Exit code | Report |
|---|---|---|---|---|---|---|---|---|---|---|
| normal-checkout | 2.22.0 | in-memory | control | fail 0 of 30 | fail 0 of 30 | fail 0 of 30 | fail 0 of 30 | 0 | 0 | [report](normal-checkout/2.22.0/report.md) |
| normal-checkout | 2.21.2 | in-memory | control | fail 0 of 30 | fail 0 of 30 | fail 0 of 30 | fail 0 of 30 | 0 | 0 | [report](normal-checkout/2.21.2/report.md) |
| normal-checkout | 2.19.0 | in-memory | control | fail 0 of 30 | fail 0 of 30 | fail 0 of 30 | fail 0 of 30 | 0 | 0 | [report](normal-checkout/2.19.0/report.md) |
| normal-checkout-two-servers | 2.22.0 | in-memory | control | fail 0 of 30 | fail 0 of 30 | fail 0 of 30 | fail 0 of 30 | 0 | 0 | [report](normal-checkout-two-servers/2.22.0/report.md) |
| normal-checkout-two-servers | 2.21.2 | in-memory | control | fail 0 of 30 | fail 0 of 30 | fail 0 of 30 | fail 0 of 30 | 0 | 0 | [report](normal-checkout-two-servers/2.21.2/report.md) |
| normal-checkout-two-servers | 2.19.0 | in-memory | control | fail 0 of 30 | fail 0 of 30 | fail 0 of 30 | fail 0 of 30 | 0 | 0 | [report](normal-checkout-two-servers/2.19.0/report.md) |
| normal-checkout-two-servers-redis-locking | 2.22.0 | redis | control | fail 0 of 30 | fail 0 of 30 | fail 0 of 30 | fail 0 of 30 | 0 | 0 | [report](normal-checkout-two-servers-redis-locking/2.22.0/report.md) |
| normal-checkout-two-servers-redis-locking | 2.21.2 | redis | control | fail 0 of 30 | fail 0 of 30 | fail 0 of 30 | fail 0 of 30 | 0 | 0 | [report](normal-checkout-two-servers-redis-locking/2.21.2/report.md) |
| normal-checkout-two-servers-redis-locking | 2.19.0 | redis | control | fail 0 of 30 | fail 0 of 30 | fail 0 of 30 | fail 0 of 30 | 0 | 0 | [report](normal-checkout-two-servers-redis-locking/2.19.0/report.md) |
| duplicate-complete | 2.22.0 | in-memory | control | fail 0 of 30 | fail 0 of 30 | fail 0 of 30 | fail 0 of 30 | 0 | 0 | [report](duplicate-complete/2.22.0/report.md) |
| duplicate-complete | 2.21.2 | in-memory | control | fail 0 of 30 | fail 0 of 30 | fail 0 of 30 | fail 0 of 30 | 0 | 0 | [report](duplicate-complete/2.21.2/report.md) |
| duplicate-complete | 2.19.0 | in-memory | control | fail 0 of 30 | fail 0 of 30 | fail 0 of 30 | fail 0 of 30 | 0 | 0 | [report](duplicate-complete/2.19.0/report.md) |
| duplicate-complete-two-servers | 2.22.0 | in-memory | control | fail 0 of 30 | fail 0 of 30 | fail 0 of 30 | fail 0 of 30 | 0 | 0 | [report](duplicate-complete-two-servers/2.22.0/report.md) |
| duplicate-complete-two-servers | 2.21.2 | in-memory | control | fail 0 of 30 | fail 0 of 30 | fail 0 of 30 | fail 0 of 30 | 0 | 0 | [report](duplicate-complete-two-servers/2.21.2/report.md) |
| duplicate-complete-two-servers | 2.19.0 | in-memory | control | fail 0 of 30 | fail 0 of 30 | fail 0 of 30 | fail 0 of 30 | 0 | 0 | [report](duplicate-complete-two-servers/2.19.0/report.md) |
| duplicate-complete-two-servers-redis-locking | 2.22.0 | redis | control | fail 0 of 30 | fail 0 of 30 | fail 0 of 30 | fail 0 of 30 | 0 | 0 | [report](duplicate-complete-two-servers-redis-locking/2.22.0/report.md) |
| duplicate-complete-two-servers-redis-locking | 2.21.2 | redis | control | fail 0 of 30 | fail 0 of 30 | fail 0 of 30 | fail 0 of 30 | 0 | 0 | [report](duplicate-complete-two-servers-redis-locking/2.21.2/report.md) |
| duplicate-complete-two-servers-redis-locking | 2.19.0 | redis | control | fail 0 of 30 | fail 0 of 30 | fail 0 of 30 | fail 0 of 30 | 0 | 0 | [report](duplicate-complete-two-servers-redis-locking/2.19.0/report.md) |
| last-unit-one-server | 2.22.0 | in-memory | control | fail 0 of 30 | fail 0 of 30 | fail 0 of 30 | fail 0 of 30 | 0 | 0 | [report](last-unit-one-server/2.22.0/report.md) |
| last-unit-one-server | 2.21.2 | in-memory | control | fail 0 of 30 | fail 0 of 30 | fail 0 of 30 | fail 0 of 30 | 0 | 0 | [report](last-unit-one-server/2.21.2/report.md) |
| last-unit-one-server | 2.19.0 | in-memory | control | fail 0 of 30 | fail 0 of 30 | fail 0 of 30 | fail 0 of 30 | 0 | 0 | [report](last-unit-one-server/2.19.0/report.md) |
| counter-one-server | 2.22.0 | in-memory | control | fail 0 of 30 | fail 0 of 30 | fail 0 of 30 | fail 0 of 30 | 0 | 0 | [report](counter-one-server/2.22.0/report.md) |
| counter-one-server | 2.21.2 | in-memory | control | fail 0 of 30 | fail 0 of 30 | fail 0 of 30 | fail 0 of 30 | 0 | 0 | [report](counter-one-server/2.21.2/report.md) |
| counter-one-server | 2.19.0 | in-memory | control | fail 0 of 30 | fail 0 of 30 | fail 0 of 30 | fail 0 of 30 | 0 | 0 | [report](counter-one-server/2.19.0/report.md) |
| single-cart-over-demand | 2.22.0 | in-memory | [#16502](https://github.com/medusajs/medusa/issues/16502) | fail 30 of 30 | fail 0 of 30 | fail 0 of 30 | fail 0 of 30 | 0 | 1 | [report](single-cart-over-demand/2.22.0/report.md) |
| single-cart-over-demand | 2.21.2 | in-memory | [#16502](https://github.com/medusajs/medusa/issues/16502) | fail 30 of 30 | fail 0 of 30 | fail 0 of 30 | fail 0 of 30 | 0 | 1 | [report](single-cart-over-demand/2.21.2/report.md) |
| single-cart-over-demand | 2.19.0 | in-memory | [#16502](https://github.com/medusajs/medusa/issues/16502) | fail 30 of 30 | fail 0 of 30 | fail 0 of 30 | fail 0 of 30 | 0 | 1 | [report](single-cart-over-demand/2.19.0/report.md) |
| single-cart-over-demand-stock-drop | 2.22.0 | in-memory | [#16502](https://github.com/medusajs/medusa/issues/16502), tracked in [#17436](https://github.com/medusajs/medusa/issues/17436) | fail 30 of 30 | fail 0 of 30 | fail 0 of 30 | fail 0 of 30 | 0 | 1 | [report](single-cart-over-demand-stock-drop/2.22.0/report.md) |
| single-cart-over-demand-stock-drop | 2.21.2 | in-memory | [#16502](https://github.com/medusajs/medusa/issues/16502), tracked in [#17436](https://github.com/medusajs/medusa/issues/17436) | fail 30 of 30 | fail 0 of 30 | fail 0 of 30 | fail 0 of 30 | 0 | 1 | [report](single-cart-over-demand-stock-drop/2.21.2/report.md) |
| single-cart-over-demand-stock-drop | 2.19.0 | in-memory | [#16502](https://github.com/medusajs/medusa/issues/16502), tracked in [#17436](https://github.com/medusajs/medusa/issues/17436) | fail 30 of 30 | fail 0 of 30 | fail 0 of 30 | fail 0 of 30 | 0 | 1 | [report](single-cart-over-demand-stock-drop/2.19.0/report.md) |
| two-carts-shared-stock | 2.22.0 | in-memory | [#17032](https://github.com/medusajs/medusa/issues/17032) | fail 15 of 30 | fail 0 of 30 | fail 0 of 30 | fail 0 of 30 | 0 | 1 | [report](two-carts-shared-stock/2.22.0/report.md) |
| two-carts-shared-stock | 2.21.2 | in-memory | [#17032](https://github.com/medusajs/medusa/issues/17032) | fail 14 of 30 | fail 0 of 30 | fail 0 of 30 | fail 0 of 30 | 0 | 1 | [report](two-carts-shared-stock/2.21.2/report.md) |
| two-carts-shared-stock | 2.19.0 | in-memory | [#17032](https://github.com/medusajs/medusa/issues/17032) | fail 13 of 30 | fail 0 of 30 | fail 0 of 30 | fail 0 of 30 | 0 | 1 | [report](two-carts-shared-stock/2.19.0/report.md) |
| two-carts-a-then-b | 2.22.0 | in-memory | order control for [#17032](https://github.com/medusajs/medusa/issues/17032) | fail 30 of 30 | fail 0 of 30 | fail 0 of 30 | fail 0 of 30 | 0 | 1 | [report](two-carts-a-then-b/2.22.0/report.md) |
| two-carts-a-then-b | 2.21.2 | in-memory | order control for [#17032](https://github.com/medusajs/medusa/issues/17032) | fail 30 of 30 | fail 0 of 30 | fail 0 of 30 | fail 0 of 30 | 0 | 1 | [report](two-carts-a-then-b/2.21.2/report.md) |
| two-carts-a-then-b | 2.19.0 | in-memory | order control for [#17032](https://github.com/medusajs/medusa/issues/17032) | fail 30 of 30 | fail 0 of 30 | fail 0 of 30 | fail 0 of 30 | 0 | 1 | [report](two-carts-a-then-b/2.19.0/report.md) |
| two-carts-b-then-a | 2.22.0 | in-memory | order control for [#17032](https://github.com/medusajs/medusa/issues/17032) | fail 0 of 30 | fail 0 of 30 | fail 0 of 30 | fail 0 of 30 | 0 | 0 | [report](two-carts-b-then-a/2.22.0/report.md) |
| two-carts-b-then-a | 2.21.2 | in-memory | order control for [#17032](https://github.com/medusajs/medusa/issues/17032) | fail 0 of 30 | fail 0 of 30 | fail 0 of 30 | fail 0 of 30 | 0 | 0 | [report](two-carts-b-then-a/2.21.2/report.md) |
| two-carts-b-then-a | 2.19.0 | in-memory | order control for [#17032](https://github.com/medusajs/medusa/issues/17032) | fail 0 of 30 | fail 0 of 30 | fail 0 of 30 | fail 0 of 30 | 0 | 0 | [report](two-carts-b-then-a/2.19.0/report.md) |
| two-carts-shared-stock-two-servers | 2.22.0 | in-memory | [#17032](https://github.com/medusajs/medusa/issues/17032) | fail 30 of 30 | fail 30 of 30 | fail 0 of 30 | fail 0 of 30 | 0 | 1 | [report](two-carts-shared-stock-two-servers/2.22.0/report.md) |
| two-carts-shared-stock-two-servers | 2.21.2 | in-memory | [#17032](https://github.com/medusajs/medusa/issues/17032) | fail 30 of 30 | fail 30 of 30 | fail 0 of 30 | fail 0 of 30 | 0 | 1 | [report](two-carts-shared-stock-two-servers/2.21.2/report.md) |
| two-carts-shared-stock-two-servers | 2.19.0 | in-memory | [#17032](https://github.com/medusajs/medusa/issues/17032) | fail 30 of 30 | fail 29 of 30 | fail 0 of 30 | fail 0 of 30 | 0 | 1 | [report](two-carts-shared-stock-two-servers/2.19.0/report.md) |
| two-carts-shared-stock-two-servers-redis-locking | 2.22.0 | redis | [#17032](https://github.com/medusajs/medusa/issues/17032) | fail 24 of 30 | fail 0 of 30 | fail 0 of 30 | fail 0 of 30 | 0 | 1 | [report](two-carts-shared-stock-two-servers-redis-locking/2.22.0/report.md) |
| two-carts-shared-stock-two-servers-redis-locking | 2.21.2 | redis | [#17032](https://github.com/medusajs/medusa/issues/17032) | fail 25 of 30 | fail 0 of 30 | fail 0 of 30 | fail 0 of 30 | 0 | 1 | [report](two-carts-shared-stock-two-servers-redis-locking/2.21.2/report.md) |
| two-carts-shared-stock-two-servers-redis-locking | 2.19.0 | redis | [#17032](https://github.com/medusajs/medusa/issues/17032) | fail 29 of 30 | fail 0 of 30 | fail 0 of 30 | fail 0 of 30 | 0 | 1 | [report](two-carts-shared-stock-two-servers-redis-locking/2.19.0/report.md) |
| last-unit-two-servers | 2.22.0 | in-memory | [#17032](https://github.com/medusajs/medusa/issues/17032) | fail 29 of 30 | fail 29 of 30 | fail 0 of 30 | fail 0 of 30 | 0 | 1 | [report](last-unit-two-servers/2.22.0/report.md) |
| last-unit-two-servers | 2.21.2 | in-memory | [#17032](https://github.com/medusajs/medusa/issues/17032) | fail 28 of 30 | fail 28 of 30 | fail 0 of 30 | fail 0 of 30 | 0 | 1 | [report](last-unit-two-servers/2.21.2/report.md) |
| last-unit-two-servers | 2.19.0 | in-memory | [#17032](https://github.com/medusajs/medusa/issues/17032) | fail 30 of 30 | fail 30 of 30 | fail 0 of 30 | fail 0 of 30 | 0 | 1 | [report](last-unit-two-servers/2.19.0/report.md) |
| last-unit-two-servers-redis-locking | 2.22.0 | redis | [#17032](https://github.com/medusajs/medusa/issues/17032) | fail 0 of 30 | fail 0 of 30 | fail 0 of 30 | fail 0 of 30 | 0 | 0 | [report](last-unit-two-servers-redis-locking/2.22.0/report.md) |
| last-unit-two-servers-redis-locking | 2.21.2 | redis | [#17032](https://github.com/medusajs/medusa/issues/17032) | fail 0 of 30 | fail 0 of 30 | fail 0 of 30 | fail 0 of 30 | 0 | 0 | [report](last-unit-two-servers-redis-locking/2.21.2/report.md) |
| last-unit-two-servers-redis-locking | 2.19.0 | redis | [#17032](https://github.com/medusajs/medusa/issues/17032) | fail 0 of 30 | fail 0 of 30 | fail 0 of 30 | fail 0 of 30 | 0 | 0 | [report](last-unit-two-servers-redis-locking/2.19.0/report.md) |
| counter-two-servers | 2.22.0 | in-memory | [#17032](https://github.com/medusajs/medusa/issues/17032) | fail 0 of 30 | fail 30 of 30 | fail 0 of 30 | fail 0 of 30 | 0 | 1 | [report](counter-two-servers/2.22.0/report.md) |
| counter-two-servers | 2.21.2 | in-memory | [#17032](https://github.com/medusajs/medusa/issues/17032) | fail 0 of 30 | fail 30 of 30 | fail 0 of 30 | fail 0 of 30 | 0 | 1 | [report](counter-two-servers/2.21.2/report.md) |
| counter-two-servers | 2.19.0 | in-memory | [#17032](https://github.com/medusajs/medusa/issues/17032) | fail 0 of 30 | fail 30 of 30 | fail 0 of 30 | fail 0 of 30 | 0 | 1 | [report](counter-two-servers/2.19.0/report.md) |
| counter-two-servers-redis-locking | 2.22.0 | redis | [#17032](https://github.com/medusajs/medusa/issues/17032) | fail 0 of 30 | fail 0 of 30 | fail 0 of 30 | fail 0 of 30 | 0 | 0 | [report](counter-two-servers-redis-locking/2.22.0/report.md) |
| counter-two-servers-redis-locking | 2.21.2 | redis | [#17032](https://github.com/medusajs/medusa/issues/17032) | fail 0 of 30 | fail 0 of 30 | fail 0 of 30 | fail 0 of 30 | 0 | 0 | [report](counter-two-servers-redis-locking/2.21.2/report.md) |
| counter-two-servers-redis-locking | 2.19.0 | redis | [#17032](https://github.com/medusajs/medusa/issues/17032) | fail 0 of 30 | fail 0 of 30 | fail 0 of 30 | fail 0 of 30 | 0 | 0 | [report](counter-two-servers-redis-locking/2.19.0/report.md) |
| many-carts-last-units | 2.22.0 | in-memory | [#16825](https://github.com/medusajs/medusa/issues/16825) | fail 0 of 30 | fail 0 of 30 | fail 0 of 30 | fail 0 of 30 | 0 | 0 | [report](many-carts-last-units/2.22.0/report.md) |
| many-carts-last-units | 2.21.2 | in-memory | [#16825](https://github.com/medusajs/medusa/issues/16825) | fail 0 of 30 | fail 0 of 30 | fail 0 of 30 | fail 0 of 30 | 0 | 0 | [report](many-carts-last-units/2.21.2/report.md) |
| many-carts-last-units | 2.19.0 | in-memory | [#16825](https://github.com/medusajs/medusa/issues/16825) | fail 0 of 30 | fail 0 of 30 | fail 0 of 30 | fail 0 of 30 | 0 | 0 | [report](many-carts-last-units/2.19.0/report.md) |

## Build variant pr-16961

`pr-16961` is Medusa 2.22.0 with the source change of [medusajs/medusa#16961](https://github.com/medusajs/medusa/pull/16961), "fix(inventory, core-flows): validate cumulative demand for shared inventory items", by pqa-labs. It is a community pull request for [#16502](https://github.com/medusajs/medusa/issues/16502) that was closed without being merged. Its head commit is `5ac8ae437ee0b7771a945b5b30f265b76f214172`, one commit on `340da77f722a5a53c48bf35276f96ec261a65736`. The variant applies its change to the two non-test source files it touches, `packages/modules/inventory/src/services/inventory-module.ts` (`ensureInventoryLevels` counts what earlier rows of the same call already took) and `packages/core/core-flows/src/cart/steps/confirm-inventory.ts` (the cart's inventory confirmation sums lines that share an inventory item). Both files are the same at that base commit and at `v2.22.0`, and the change applied cleanly to `v2.22.0`.

How the files were built and checked, with [scripts/build-variant.sh](../scripts/build-variant.sh) (see [docs/variant-format.md](../docs/variant-format.md)):

- `@medusajs/inventory` and `@medusajs/core-flows` were compiled from `v2.22.0` inside the Medusa 2.22.0 image, with `tsc --build` and, for `@medusajs/inventory`, the `tsc-alias` step of its build script, using TypeScript 5.9.3, tsc-alias 1.8.16, @types/node 20.19.27, @types/jest 30.0.0 and expect-type 0.20.0, the versions Medusa's `yarn.lock` resolves at `v2.22.0`.
- Unpatched, all 868 published `.js` files of the two packages (18 and 850) compiled byte-identical to the files in the Medusa 2.22.0 image. The compile also produced 7 test and fixture files that the packages do not publish.
- Patched, exactly two `.js` files changed. They replace the published files in the variant image:
  - `@medusajs/inventory/dist/services/inventory-module.js`: sha256 `8e150f6b9de7bc64f431e91a803716646a782379d3cc993a31f301056d5ac919`, replacing `6d2643ef94e7ffe20fb2b4a35f8f1de503ad4429c85c64cc4262ceae2a776c58`
  - `@medusajs/core-flows/dist/cart/steps/confirm-inventory.js`: sha256 `2af8432c719836364a143d9fe7f4ee6f9aba17062d555e2fdace47315c11c569`, replacing `01a7cf229b6854a7840207c6ba62234bbdedc651adfd50276536e06d6e46134d`
- Every run read both files' sha256 inside each running Medusa container, and every value matched.

Each cell is the scenario's fail count on plain 2.22.0, then on 2.22.0 with pr-16961, out of the runs recorded. The plain 2.22.0 numbers are the published run sets in the main table (from the Medusa 2.22.0, Redis locking and pr-16961 campaigns below). The exit codes are in the same order.

| Scenario | Targets | no_oversell | reserved_matches_reservations | one_order_per_cart | no_orphaned_orders | Exit code | Reports |
|---|---|---|---|---|---|---|---|
| single-cart-over-demand-stock-drop | [#16502](https://github.com/medusajs/medusa/issues/16502), tracked in [#17436](https://github.com/medusajs/medusa/issues/17436) | 30 of 30 / 0 of 30 | 0 of 30 / 0 of 30 | 0 of 30 / 0 of 30 | 0 of 30 / 0 of 30 | 1 / 0 | [2.22.0](single-cart-over-demand-stock-drop/2.22.0/report.md), [pr-16961](single-cart-over-demand-stock-drop/2.22.0-pr-16961/report.md) |
| two-carts-shared-stock | [#17032](https://github.com/medusajs/medusa/issues/17032) | 15 of 30 / 0 of 30 | 0 of 30 / 0 of 30 | 0 of 30 / 0 of 30 | 0 of 30 / 0 of 30 | 1 / 0 | [2.22.0](two-carts-shared-stock/2.22.0/report.md), [pr-16961](two-carts-shared-stock/2.22.0-pr-16961/report.md) |
| two-carts-a-then-b | order control for [#17032](https://github.com/medusajs/medusa/issues/17032) | 30 of 30 / 0 of 30 | 0 of 30 / 0 of 30 | 0 of 30 / 0 of 30 | 0 of 30 / 0 of 30 | 1 / 0 | [2.22.0](two-carts-a-then-b/2.22.0/report.md), [pr-16961](two-carts-a-then-b/2.22.0-pr-16961/report.md) |
| two-carts-b-then-a | order control for [#17032](https://github.com/medusajs/medusa/issues/17032) | 0 of 30 / 0 of 30 | 0 of 30 / 0 of 30 | 0 of 30 / 0 of 30 | 0 of 30 / 0 of 30 | 0 / 0 | [2.22.0](two-carts-b-then-a/2.22.0/report.md), [pr-16961](two-carts-b-then-a/2.22.0-pr-16961/report.md) |
| two-carts-shared-stock-two-servers | [#17032](https://github.com/medusajs/medusa/issues/17032) | 30 of 30 / 29 of 30 | 30 of 30 / 29 of 30 | 0 of 30 / 0 of 30 | 0 of 30 / 0 of 30 | 1 / 1 | [2.22.0](two-carts-shared-stock-two-servers/2.22.0/report.md), [pr-16961](two-carts-shared-stock-two-servers/2.22.0-pr-16961/report.md) |
| two-carts-shared-stock-two-servers-redis-locking | [#17032](https://github.com/medusajs/medusa/issues/17032) | 24 of 30 / 0 of 30 | 0 of 30 / 0 of 30 | 0 of 30 / 0 of 30 | 0 of 30 / 0 of 30 | 1 / 0 | [2.22.0](two-carts-shared-stock-two-servers-redis-locking/2.22.0/report.md), [pr-16961](two-carts-shared-stock-two-servers-redis-locking/2.22.0-pr-16961/report.md) |
| last-unit-two-servers | [#17032](https://github.com/medusajs/medusa/issues/17032) | 29 of 30 / 30 of 30 | 29 of 30 / 30 of 30 | 0 of 30 / 0 of 30 | 0 of 30 / 0 of 30 | 1 / 1 | [2.22.0](last-unit-two-servers/2.22.0/report.md), [pr-16961](last-unit-two-servers/2.22.0-pr-16961/report.md) |
| counter-two-servers | [#17032](https://github.com/medusajs/medusa/issues/17032) | 0 of 30 / 0 of 30 | 30 of 30 / 30 of 30 | 0 of 30 / 0 of 30 | 0 of 30 / 0 of 30 | 1 / 1 | [2.22.0](counter-two-servers/2.22.0/report.md), [pr-16961](counter-two-servers/2.22.0-pr-16961/report.md) |
| normal-checkout | control | 0 of 30 / 0 of 30 | 0 of 30 / 0 of 30 | 0 of 30 / 0 of 30 | 0 of 30 / 0 of 30 | 0 / 0 | [2.22.0](normal-checkout/2.22.0/report.md), [pr-16961](normal-checkout/2.22.0-pr-16961/report.md) |
| normal-checkout-two-servers | control | 0 of 30 / 0 of 30 | 0 of 30 / 0 of 30 | 0 of 30 / 0 of 30 | 0 of 30 / 0 of 30 | 0 / 0 | [2.22.0](normal-checkout-two-servers/2.22.0/report.md), [pr-16961](normal-checkout-two-servers/2.22.0-pr-16961/report.md) |
| duplicate-complete | control | 0 of 30 / 0 of 30 | 0 of 30 / 0 of 30 | 0 of 30 / 0 of 30 | 0 of 30 / 0 of 30 | 0 / 0 | [2.22.0](duplicate-complete/2.22.0/report.md), [pr-16961](duplicate-complete/2.22.0-pr-16961/report.md) |
| last-unit-one-server | control | 0 of 30 / 0 of 30 | 0 of 30 / 0 of 30 | 0 of 30 / 0 of 30 | 0 of 30 / 0 of 30 | 0 / 0 | [2.22.0](last-unit-one-server/2.22.0/report.md), [pr-16961](last-unit-one-server/2.22.0-pr-16961/report.md) |
| counter-one-server | control | 0 of 30 / 0 of 30 | 0 of 30 / 0 of 30 | 0 of 30 / 0 of 30 | 0 of 30 / 0 of 30 | 0 / 0 | [2.22.0](counter-one-server/2.22.0/report.md), [pr-16961](counter-one-server/2.22.0-pr-16961/report.md) |

On these runs:

- single-cart-over-demand-stock-drop, two-carts-shared-stock, two-carts-a-then-b and two-carts-shared-stock-two-servers-redis-locking failed no check with pr-16961. In each of their runs one completion was rejected with "Not enough stock available for item ... at location ...".
- In two-carts-shared-stock, cart a's complete returned first in 14 of 30 runs, and cart b's complete was rejected in all 14. In the other 16, cart a's was rejected. In two-carts-a-then-b, which completes cart a and then cart b one after the other, cart b's complete was rejected in all 30 runs.
- two-carts-shared-stock-two-servers, last-unit-two-servers and counter-two-servers, which complete on two servers with Medusa's in-memory locking, failed the same checks as on plain 2.22.0. two-carts-shared-stock-two-servers failed no_oversell in 29 of 30 runs; in run 16, cart a's complete returned first and cart b's was rejected.
- With pr-16961, two-carts-shared-stock-two-servers-redis-locking failed no check, where plain 2.22.0 with Redis locking failed no_oversell in 24 of 30. Cart a's complete returned first in 27 runs: in 25 cart b's was rejected, and in runs 22 and 24 cart a's own complete was rejected and cart b placed the order.
- The controls (normal-checkout, normal-checkout-two-servers, duplicate-complete, last-unit-one-server, counter-one-server and two-carts-b-then-a) failed no check, as on plain 2.22.0.
- On pr-16961, Oversold builds single-cart-over-demand's cart without an error: it adds one line per call, and both calls returned 200. The complete was then rejected, in 3 of 3 runs that were not published.

## Campaigns

### Build variant pr-16961 and single-cart-over-demand-stock-drop

- Oversold commit: `012083b1feeee16e290b8050f53e76b51f53bf72` (no uncommitted changes in any history)
- Images, each labeled with the SHA-256 of the files it was built from (`oversold.image-inputs`):
  - Medusa 2.22.0: `sha256:d2292583825ecaba116d2d8a0dbf83d96412620224b4a90d64706b1270dd6bed`, inputs `dcc1486dc4b9b2cfaa84d6a20aa494562befef8031cf98606afef5c2a168a728`
  - Medusa 2.21.2: `sha256:632cf6bf70947658bc55cb1cb6df97a9338d3e341869b28f29421458eabea8c0`, inputs `896481885944e7045d19b0e0a654735d26eff398726dd62b3c7f43c32ec85be3`
  - Medusa 2.19.0: `sha256:952c9571aab805c2a1cc1de343c0afe9bc688d28d5c66558d94b4711308eefdd`, inputs `c915b72cca1362431054f7b7ae060099190915f80440e2d1d07b917807b5a875`
  - Medusa 2.22.0 with pr-16961: `sha256:48986d179a7b342c15456b7bbe655d539d4dd84ee14085111fecaebf900e3cce`, inputs `d7306668cba9594398c0dc9e3289bbbadeabad6ad7c20c9292104713ebd68257`, template database `medusa_template_2_22_0_pr_16961`
- Postgres 15.19, Medusa's default in-memory modules except for two-carts-shared-stock-two-servers-redis-locking (Redis 7.4.11 locking), one or two Medusa servers on one database, local Docker on one machine
- Runs: 480 in total, single-cart-over-demand-stock-drop on each version and 13 scenarios on 2.22.0 with pr-16961, from 2026-10-29T18:21:38.237296000Z to 2026-10-29T19:06:47.480300000Z
- Every run started from a fresh copy of the template database for its image, with the Medusa servers restarted.

### Redis locking

- Oversold commit: `73e03abeee95629300f422b57b9750585cbc18de` (no uncommitted changes in any history)
- Medusa images, rebuilt after `medusa-config.ts` gained the locking option: 2.22.0 `sha256:0c78357f67a55dfb7ba59325bb9bebf985553495a3d7b3cf2b8b4936ef0e2814`, 2.21.2 `sha256:56be8697b2ac544854583fd3dc79d02f6e48990ea1d51076281c8baf67da63cb`, 2.19.0 `sha256:6a21dbc25ef2a92d21a72db65f885855c6309a5861e9f5b7d3487f723d87d7c6`
- Redis 7.4.11 (`redis:7.4.11-alpine`) with Medusa's Redis locking provider on both servers, every other Medusa module in-memory, Postgres 15.19, two Medusa servers on one database, local Docker on one machine
- Runs: 450 in total, the five `-redis-locking` scenarios on each version, from 2026-10-26T00:06:01.036042000Z to 2026-10-26T00:58:16.129553000Z
- Every run started from a fresh copy of the template database for its version, with Redis emptied and the Medusa servers restarted.
- Scripted Redis lock calls during the race, per run: 4 for normal-checkout-two-servers-redis-locking, 7 for duplicate-complete-two-servers-redis-locking, 8 to 10 for two-carts-shared-stock-two-servers-redis-locking, 9 to 10 for last-unit-two-servers-redis-locking, 59 to 67 for counter-two-servers-redis-locking. Each run's count and the Redis command stats it comes from are in its history.

### Medusa 2.22.0

- Oversold commit: `e4cd250e2e47cfaafa02f8d2bcda31e756337f90` (no uncommitted changes in any history)
- Medusa 2.22.0 image: `sha256:3c8395e320a647127fcbbb214894cd58cbd71dd0d8cb1343d9742cd80dc6bab6`
- Postgres 15.19, Medusa's default in-memory modules, one or two Medusa servers on one database, local Docker on one machine
- Runs: 420 in total, from 2026-10-25T07:54:23.520949000Z to 2026-10-25T08:41:28.142611000Z
- Every run started from a fresh copy of the template database for its version, with the Medusa servers restarted.

### Medusa 2.21.2 and 2.19.0

- Oversold commit: `ce1011c42d32c3643a69c3c2b91e9b20fd5c550b` (no uncommitted changes in any history)
- Medusa 2.21.2 image: `sha256:8518b2338505f0e1f2021b89064b69f03b9450e522af55d20f2c3b2c83a1111b`
- Medusa 2.19.0 image: `sha256:24f515d6ce8af95d6edacd2c62229186651dbce21aede31ea271921a3c7b8c0d`
- Postgres 15.19, Medusa's default in-memory modules, one or two Medusa servers on one database, local Docker on one machine
- Runs: 840 in total, from 2026-10-20T02:33:18.429441000Z to 2026-10-20T04:05:49.559132000Z
- Every run started from a fresh copy of the template database for its version, with the Medusa servers restarted.

## How to recheck

Build Oversold, then recompute every verdict and report from the saved histories. This needs no Docker and writes nothing.

```sh
make build
for d in results/*/*/; do ./bin/oversold check --verify "$d"; done
```

`oversold check --verify` exits 0 only when the stored verdicts and the report match the ones it recomputes with the current check rules, and 1 when anything differs. `oversold check <folder>` prints every run's verdicts and rewrites that folder's report.
