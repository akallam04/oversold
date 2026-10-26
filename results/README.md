# Results

Measured runs of every scenario in [scenarios/medusa/](../scenarios/medusa/), 30 runs each, on Medusa 2.22.0 (released Oct 21, 2026), 2.21.2 (released Sep 28, 2026) and 2.19.0 (released Aug 13, 2026). Each folder holds the scenario file that was run, `report.json`, `report.md` and every run's history, gzipped. The checks are defined in [docs/checks.md](../docs/checks.md), and the history format in [docs/history-format.md](../docs/history-format.md).

Each check cell counts the runs where that check failed, out of the runs recorded. The exit code is the one `oversold run` gave for that run set: 0 when every check passed in every run, 1 when any check failed in any run.

On 2.22.0, the same checks failed in the same scenarios as on 2.21.2. The fail counts differ only for two-carts-shared-stock (15 of 30 on 2.22.0, 14 of 30 on 2.21.2) and last-unit-two-servers (29 of 30 on 2.22.0, 28 of 30 on 2.21.2).

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

## Campaigns

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
