# Results

Measured runs of every scenario in [scenarios/medusa/](../scenarios/medusa/), 30 runs each, on Medusa 2.22.0 (released Oct 21, 2026), 2.21.2 (released Sep 28, 2026) and 2.19.0 (released Aug 13, 2026). Each folder holds the scenario file that was run, `report.json`, `report.md` and every run's history, gzipped. The checks are defined in [docs/checks.md](../docs/checks.md), and the history format in [docs/history-format.md](../docs/history-format.md).

Each check cell counts the runs where that check failed, out of the runs recorded. The folders named `2.22.0-pr-16961` and `2.22.0-pr-17420` hold runs of the pr-16961 and pr-17420 build variants, described in their own sections below ([pr-16961](#build-variant-pr-16961), [pr-17420](#build-variant-pr-17420)). The folders named `2.22.0-2026-11-13` hold a rerun of plain 2.22.0 on the same image the pr-17420 variant was built on, compared in the pr-17420 section. None of these are rows of the main table. The exit code is the one `oversold run` gave for that run set: 0 when every check passed in every run, 1 when any check failed in any run.

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

## Bug scenarios side by side

Each cell is the fail count of no_oversell / reserved_matches_reservations / one_order_per_cart / no_orphaned_orders, out of 30 runs, on Medusa 2.22.0. "2.22.0, October" shows the plain run sets in the main table, on images built in October. "2.22.0, Nov 13" shows the plain rerun on the same Medusa 2.22.0 image the pr-17420 variant was built on. "2.22.0 with Redis locking" is the scenario's `-redis-locking` copy, which exists only for the two-server scenarios. pr-16961 and pr-17420 are the build variants described below: Medusa 2.22.0 with the source change of a community pull request that was not merged, with Medusa's default in-memory locking. pr-16961 also ran two-carts-shared-stock-two-servers-redis-locking and failed no check there.

| Scenario | Targets | 2.22.0, October | 2.22.0, Nov 13 | 2.22.0 with Redis locking | pr-16961 | pr-17420 |
|---|---|---|---|---|---|---|
| single-cart-over-demand | [#16502](https://github.com/medusajs/medusa/issues/16502) | 30 / 0 / 0 / 0 of 30 | 30 / 0 / 0 / 0 of 30 | not run | 0 / 0 / 0 / 0 of 30 | 30 / 0 / 0 / 0 of 30 |
| single-cart-over-demand-stock-drop | [#16502](https://github.com/medusajs/medusa/issues/16502), tracked in [#17436](https://github.com/medusajs/medusa/issues/17436) | 30 / 0 / 0 / 0 of 30 | 30 / 0 / 0 / 0 of 30 | not run | 0 / 0 / 0 / 0 of 30 | 30 / 0 / 0 / 0 of 30 |
| two-carts-shared-stock | [#17032](https://github.com/medusajs/medusa/issues/17032) | 15 / 0 / 0 / 0 of 30 | 13 / 0 / 0 / 0 of 30 | not run | 0 / 0 / 0 / 0 of 30 | 15 / 0 / 0 / 0 of 30 |
| two-carts-shared-stock-two-servers | [#17032](https://github.com/medusajs/medusa/issues/17032) | 30 / 30 / 0 / 0 of 30 | 28 / 28 / 0 / 0 of 30 | 24 / 0 / 0 / 0 of 30 | 29 / 29 / 0 / 0 of 30 | 17 / 0 / 0 / 0 of 30 |
| last-unit-two-servers | [#17032](https://github.com/medusajs/medusa/issues/17032) | 29 / 29 / 0 / 0 of 30 | 30 / 30 / 0 / 0 of 30 | 0 / 0 / 0 / 0 of 30 | 30 / 30 / 0 / 0 of 30 | 0 / 0 / 0 / 0 of 30 |
| counter-two-servers | [#17032](https://github.com/medusajs/medusa/issues/17032) | 0 / 30 / 0 / 0 of 30 | 0 / 30 / 0 / 0 of 30 | 0 / 0 / 0 / 0 of 30 | 0 / 30 / 0 / 0 of 30 | 0 / 0 / 0 / 0 of 30 |
| many-carts-last-units | [#16825](https://github.com/medusajs/medusa/issues/16825) | 0 / 0 / 0 / 0 of 30 | 0 / 0 / 0 / 0 of 30 | not run | not run | 0 / 0 / 0 / 0 of 30 |

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
| single-cart-over-demand | [#16502](https://github.com/medusajs/medusa/issues/16502) | 30 of 30 / 0 of 30 | 0 of 30 / 0 of 30 | 0 of 30 / 0 of 30 | 0 of 30 / 0 of 30 | 1 / 0 | [2.22.0](single-cart-over-demand/2.22.0/report.md), [pr-16961](single-cart-over-demand/2.22.0-pr-16961/report.md) |
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

- single-cart-over-demand, single-cart-over-demand-stock-drop, two-carts-shared-stock, two-carts-a-then-b and two-carts-shared-stock-two-servers-redis-locking failed no check with pr-16961. In each of their runs one completion was rejected with "Not enough stock available for item ... at location ...".
- In two-carts-shared-stock, cart a's complete returned first in 14 of 30 runs, and cart b's complete was rejected in all 14. In the other 16, cart a's was rejected. In two-carts-a-then-b, which completes cart a and then cart b one after the other, cart b's complete was rejected in all 30 runs.
- two-carts-shared-stock-two-servers, last-unit-two-servers and counter-two-servers, which complete on two servers with Medusa's in-memory locking, failed the same checks as on plain 2.22.0. two-carts-shared-stock-two-servers failed no_oversell in 29 of 30 runs; in run 16, cart a's complete returned first and cart b's was rejected.
- With pr-16961, two-carts-shared-stock-two-servers-redis-locking failed no check, where plain 2.22.0 with Redis locking failed no_oversell in 24 of 30. Cart a's complete returned first in 27 runs: in 25 cart b's was rejected, and in runs 22 and 24 cart a's own complete was rejected and cart b placed the order.
- The controls (normal-checkout, normal-checkout-two-servers, duplicate-complete, last-unit-one-server, counter-one-server and two-carts-b-then-a) failed no check, as on plain 2.22.0.
- On pr-16961, Oversold builds single-cart-over-demand's cart without an error: it adds one line per call, and both calls returned 200 in all 30 runs. The complete was then rejected in all 30.

## Build variant pr-17420

`pr-17420` is Medusa 2.22.0 with the source change of [medusajs/medusa#17420](https://github.com/medusajs/medusa/pull/17420), "fix(inventory): serialize reservation mutations", by haranishi. It is a community pull request for [#16569](https://github.com/medusajs/medusa/issues/16569) and [#16488](https://github.com/medusajs/medusa/issues/16488) that was closed without being merged. Its head commit is `5332c04634e5e02ff05a5ef98a38d7c593205ee5`, two commits on `f7add009dd2f142d98090a5194750e71b2147ddf`. Its only non-test source change is in `packages/modules/inventory/src/services/inventory-module.ts`: inside the transaction, it locks the reservation rows and then the inventory level rows with `SELECT ... FOR UPDATE`, ordered by id, with a 3 second `lock_timeout`, and checks availability against the locked rows. Its other changes are tests, docs and a changeset, which the variant leaves out. The file is the same at that base commit and at `v2.22.0`, and the change applied cleanly to `v2.22.0`.

How the file was built and checked, with [scripts/build-variant.sh](../scripts/build-variant.sh):

- `@medusajs/inventory` was compiled from `v2.22.0` inside the Medusa 2.22.0 image the same way as for pr-16961, with the same TypeScript, tsc-alias, @types/node, @types/jest and expect-type versions from Medusa's `yarn.lock`.
- Unpatched, all 18 published `.js` files of the package compiled byte-identical to the files in the Medusa 2.22.0 image. The compile also produced 1 test file that the package does not publish.
- Patched, exactly one `.js` file changed. It replaces the published file in the variant image:
  - `@medusajs/inventory/dist/services/inventory-module.js`: sha256 `3744f5fc9f893e4070be02c4a74f338c67c14c5099f97e9470776ccf8df1874b`, replacing `6d2643ef94e7ffe20fb2b4a35f8f1de503ad4429c85c64cc4262ceae2a776c58`
- Every run read the file's sha256 inside each running Medusa container, and every value matched.

Each cell is the scenario's fail count out of 30 runs in three run sets, in this order: plain 2.22.0 from October (the run sets in the main table), plain 2.22.0 from Nov 13, and 2.22.0 with pr-17420. The exit codes are in the same order. The October runs used images built on `node:22.23.3-bookworm-slim` at `sha256:c3de60bf…`. The Nov 13 runs used the same Medusa 2.22.0 image as the pr-17420 runs' base (`sha256:5a6644e9…`, built on `sha256:3ed98443…`), and the pr-17420 runs used the variant image built from it.

| Scenario | Targets | no_oversell | reserved_matches_reservations | one_order_per_cart | no_orphaned_orders | Exit code | Reports |
|---|---|---|---|---|---|---|---|
| single-cart-over-demand | [#16502](https://github.com/medusajs/medusa/issues/16502) | 30 / 30 / 30 | 0 / 0 / 0 | 0 / 0 / 0 | 0 / 0 / 0 | 1 / 1 / 1 | [October](single-cart-over-demand/2.22.0/report.md), [Nov 13](single-cart-over-demand/2.22.0-2026-11-13/report.md), [pr-17420](single-cart-over-demand/2.22.0-pr-17420/report.md) |
| single-cart-over-demand-stock-drop | [#16502](https://github.com/medusajs/medusa/issues/16502), tracked in [#17436](https://github.com/medusajs/medusa/issues/17436) | 30 / 30 / 30 | 0 / 0 / 0 | 0 / 0 / 0 | 0 / 0 / 0 | 1 / 1 / 1 | [October](single-cart-over-demand-stock-drop/2.22.0/report.md), [Nov 13](single-cart-over-demand-stock-drop/2.22.0-2026-11-13/report.md), [pr-17420](single-cart-over-demand-stock-drop/2.22.0-pr-17420/report.md) |
| two-carts-shared-stock | [#17032](https://github.com/medusajs/medusa/issues/17032) | 15 / 13 / 15 | 0 / 0 / 0 | 0 / 0 / 0 | 0 / 0 / 0 | 1 / 1 / 1 | [October](two-carts-shared-stock/2.22.0/report.md), [Nov 13](two-carts-shared-stock/2.22.0-2026-11-13/report.md), [pr-17420](two-carts-shared-stock/2.22.0-pr-17420/report.md) |
| two-carts-a-then-b | order control for [#17032](https://github.com/medusajs/medusa/issues/17032) | 30 / 30 / 30 | 0 / 0 / 0 | 0 / 0 / 0 | 0 / 0 / 0 | 1 / 1 / 1 | [October](two-carts-a-then-b/2.22.0/report.md), [Nov 13](two-carts-a-then-b/2.22.0-2026-11-13/report.md), [pr-17420](two-carts-a-then-b/2.22.0-pr-17420/report.md) |
| two-carts-b-then-a | order control for [#17032](https://github.com/medusajs/medusa/issues/17032) | 0 / 0 / 0 | 0 / 0 / 0 | 0 / 0 / 0 | 0 / 0 / 0 | 0 / 0 / 0 | [October](two-carts-b-then-a/2.22.0/report.md), [Nov 13](two-carts-b-then-a/2.22.0-2026-11-13/report.md), [pr-17420](two-carts-b-then-a/2.22.0-pr-17420/report.md) |
| two-carts-shared-stock-two-servers | [#17032](https://github.com/medusajs/medusa/issues/17032) | 30 / 28 / 17 | 30 / 28 / 0 | 0 / 0 / 0 | 0 / 0 / 0 | 1 / 1 / 1 | [October](two-carts-shared-stock-two-servers/2.22.0/report.md), [Nov 13](two-carts-shared-stock-two-servers/2.22.0-2026-11-13/report.md), [pr-17420](two-carts-shared-stock-two-servers/2.22.0-pr-17420/report.md) |
| last-unit-two-servers | [#17032](https://github.com/medusajs/medusa/issues/17032) | 29 / 30 / 0 | 29 / 30 / 0 | 0 / 0 / 0 | 0 / 0 / 0 | 1 / 1 / 0 | [October](last-unit-two-servers/2.22.0/report.md), [Nov 13](last-unit-two-servers/2.22.0-2026-11-13/report.md), [pr-17420](last-unit-two-servers/2.22.0-pr-17420/report.md) |
| counter-two-servers | [#17032](https://github.com/medusajs/medusa/issues/17032) | 0 / 0 / 0 | 30 / 30 / 0 | 0 / 0 / 0 | 0 / 0 / 0 | 1 / 1 / 0 | [October](counter-two-servers/2.22.0/report.md), [Nov 13](counter-two-servers/2.22.0-2026-11-13/report.md), [pr-17420](counter-two-servers/2.22.0-pr-17420/report.md) |
| many-carts-last-units | [#16825](https://github.com/medusajs/medusa/issues/16825) | 0 / 0 / 0 | 0 / 0 / 0 | 0 / 0 / 0 | 0 / 0 / 0 | 0 / 0 / 0 | [October](many-carts-last-units/2.22.0/report.md), [Nov 13](many-carts-last-units/2.22.0-2026-11-13/report.md), [pr-17420](many-carts-last-units/2.22.0-pr-17420/report.md) |
| normal-checkout | control | 0 / 0 / 0 | 0 / 0 / 0 | 0 / 0 / 0 | 0 / 0 / 0 | 0 / 0 / 0 | [October](normal-checkout/2.22.0/report.md), [Nov 13](normal-checkout/2.22.0-2026-11-13/report.md), [pr-17420](normal-checkout/2.22.0-pr-17420/report.md) |
| normal-checkout-two-servers | control | 0 / 0 / 0 | 0 / 0 / 0 | 0 / 0 / 0 | 0 / 0 / 0 | 0 / 0 / 0 | [October](normal-checkout-two-servers/2.22.0/report.md), [Nov 13](normal-checkout-two-servers/2.22.0-2026-11-13/report.md), [pr-17420](normal-checkout-two-servers/2.22.0-pr-17420/report.md) |
| duplicate-complete | control | 0 / 0 / 0 | 0 / 0 / 0 | 0 / 0 / 0 | 0 / 0 / 0 | 0 / 0 / 0 | [October](duplicate-complete/2.22.0/report.md), [Nov 13](duplicate-complete/2.22.0-2026-11-13/report.md), [pr-17420](duplicate-complete/2.22.0-pr-17420/report.md) |
| duplicate-complete-two-servers | control | 0 / 0 / 0 | 0 / 0 / 0 | 0 / 0 / 0 | 0 / 0 / 0 | 0 / 0 / 0 | [October](duplicate-complete-two-servers/2.22.0/report.md), [Nov 13](duplicate-complete-two-servers/2.22.0-2026-11-13/report.md), [pr-17420](duplicate-complete-two-servers/2.22.0-pr-17420/report.md) |
| last-unit-one-server | control | 0 / 0 / 0 | 0 / 0 / 0 | 0 / 0 / 0 | 0 / 0 / 0 | 0 / 0 / 0 | [October](last-unit-one-server/2.22.0/report.md), [Nov 13](last-unit-one-server/2.22.0-2026-11-13/report.md), [pr-17420](last-unit-one-server/2.22.0-pr-17420/report.md) |
| counter-one-server | control | 0 / 0 / 0 | 0 / 0 / 0 | 0 / 0 / 0 | 0 / 0 / 0 | 0 / 0 / 0 | [October](counter-one-server/2.22.0/report.md), [Nov 13](counter-one-server/2.22.0-2026-11-13/report.md), [pr-17420](counter-one-server/2.22.0-pr-17420/report.md) |

On these runs:

- last-unit-two-servers and counter-two-servers, which complete on two servers with Medusa's in-memory locking, failed no check with pr-17420. In last-unit-two-servers, every run placed one order and the other completion was rejected with "Not enough stock available for item ... at location ...", where plain 2.22.0 on the same image placed two orders for the one SOLO unit in 30 of 30 runs. In counter-two-servers, every run placed all ten orders, and the WIDGET reserved quantity matched the live reservations in all 30 runs. On plain 2.22.0 it was 3 to 5 units lower than the live reservations, median 4, in October and on Nov 13 alike.
- two-carts-shared-stock-two-servers failed no_oversell in 17 of 30 runs and reserved_matches_reservations in none, where plain 2.22.0 failed both in 28 of 30 on the same image (30 of 30 in October). Cart a's complete returned first in 19 runs. In 17 of them, cart b's complete also placed an order. In runs 1 and 4, cart a's own complete was rejected and cart b placed the order. In the other 11 runs, cart b's complete returned first and cart a's was rejected.
- single-cart-over-demand, single-cart-over-demand-stock-drop, two-carts-shared-stock and two-carts-a-then-b, which oversell on one server, failed no_oversell with pr-17420 about as often as on plain 2.22.0: 30, 30, 15 and 30 of 30, against 30, 30, 13 and 30 on the same image. In two-carts-shared-stock, cart a's complete returned first in 15 runs, and cart b's complete also placed an order in all 15.
- many-carts-last-units failed no check, as on plain 2.22.0. Every run placed 2 orders and rejected the other 48 completions with "Not enough stock available for item ... at location ...". Its slowest race call took 3,010 ms, against 3,153 ms for plain 2.22.0 on the same image.
- The controls (normal-checkout, normal-checkout-two-servers, duplicate-complete, duplicate-complete-two-servers, last-unit-one-server, counter-one-server and two-carts-b-then-a) failed no check, as on plain 2.22.0 in October and on Nov 13.
- Three plain 2.22.0 counts differ between October and Nov 13: last-unit-two-servers failed no_oversell and reserved_matches_reservations in 30 of 30 (29 in October), two-carts-shared-stock-two-servers in 28 of 30 (30 in October), and two-carts-shared-stock failed no_oversell in 13 of 30 (15 in October). Every other plain count is the same. In duplicate-complete-two-servers, one of the two completes was rejected with "Cannot create multiple links between 'order' and 'payment'" in every run, as on plain 2.22.0 in all 30 runs of both plain run sets.
- No response in these runs reported a lock timeout or a deadlock.

## Campaigns

### Plain 2.22.0 on the pr-17420 base image

- Oversold commit: `338ddb5c51e0345c52b75bfd037163f68c6f449e` (no uncommitted changes in any history)
- Image: Medusa 2.22.0 `sha256:5a6644e9664ee21a174878f8c1744ac11baeb364efc666f14217b5ba274ec2d7`, inputs `dcc1486dc4b9b2cfaa84d6a20aa494562befef8031cf98606afef5c2a168a728`, the image the pr-17420 variant image was built on, on `node:22.23.3-bookworm-slim` at `sha256:3ed98443660ff48413b5af19ecefce3b9d50845a901a43420d47cc63a9fddbe9`
- Postgres 15.19 (`postgres:15.19-alpine` at `sha256:aae6554de426ae1990f1a939ff183a0299146cf7142a6f4c379c3357af9cc2d8`), Medusa's default in-memory modules, one or two Medusa servers on one database, local Docker on one machine
- Runs: 450 in total, the 15 scenarios of the pr-17420 campaign, from 2026-11-13T05:53:28.293507000Z to 2026-11-13T06:43:09.412261000Z
- Every run started from a fresh copy of the template database for its image, with the Medusa servers restarted.

### Build variant pr-17420 and single-cart-over-demand on pr-16961

- Oversold commit: `cb70f7cfddb25c7f9939feb22d16cf0e2b14efd9` (no uncommitted changes in any history)
- Images, each labeled with the SHA-256 of the files it was built from (`oversold.image-inputs`):
  - Medusa 2.22.0 with pr-17420: `sha256:969768146330a4f73b8c8f46028358e42ecc42ce4bc90645483ce70044ce0b0a`, inputs `5ec4fa86bd241130178210d59c234b1de1e6957379981b793ede2bbd53b87946`, template database `medusa_template_2_22_0_pr_17420`
  - Medusa 2.22.0 with pr-16961: `sha256:9b8df728a07597d168c453f8dde19f559cf7656483206faac8c9a513ba8ccd80`, inputs `d7306668cba9594398c0dc9e3289bbbadeabad6ad7c20c9292104713ebd68257`
- The Medusa images were rebuilt on 2026-11-11 from the same build inputs, so the 2.22.0 and pr-16961 images kept the `oversold.image-inputs` hashes recorded in the histories of the pr-16961 campaign below. Their image IDs changed, and so did the base image: the tag `node:22.23.3-bookworm-slim` resolved to `sha256:3ed98443660ff48413b5af19ecefce3b9d50845a901a43420d47cc63a9fddbe9` for the rebuild, and to `sha256:c3de60bf2f9dd0ac6370e6117950ff62d6e339527e7472301c9c78a017978392` when the images of the earlier campaigns were built.
- Postgres 15.19, Medusa's default in-memory modules, one or two Medusa servers on one database, local Docker on one machine
- Runs: 480 in total, single-cart-over-demand on 2.22.0 with pr-16961 and 15 scenarios on 2.22.0 with pr-17420, from 2026-11-12T17:07:38.340371000Z to 2026-11-12T17:58:57.501955000Z
- Every run started from a fresh copy of the template database for its image, with the Medusa servers restarted.

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
