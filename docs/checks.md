# Checks

A check reads one run's history and gives a verdict: `pass`, `fail` or `inconclusive`. Checks are pure functions over the saved history. They need no Docker and no database, so anyone can recompute them later with `oversold check <run folder>`.

Each check has a name and a version. The version starts at 1. Any change to a rule bumps that check's version and adds a line to [Version history](#version-history) saying what changed and why. Every verdict records the check version that produced it.

A scenario chooses its checks with the `checks` field. If the field is left out, all four apply.

## Terms

- **Before snapshot** and **after snapshot**: the `before` and `after` database snapshots in the history. The before snapshot is taken at the start of the race, after setup. The after snapshot is taken 2 seconds after the race ends.
- **Level**: one row of `inventory_levels`, identified by inventory item and location. It has `stocked_quantity` and `reserved_quantity`.
- **Live reservation**: one row of `live_reservations`, a `reservation_item` row that is not deleted.
- **Live reservation sum** of a level: the sum of `quantity` over live reservations with the same inventory item and location.
- **Units a line needs** for an inventory item: the line's `quantity` times the `required_quantity` for that variant and inventory item, taken from the snapshot's `variants`. A line can need units of more than one inventory item.
- **Counted orders**: orders in the after snapshot that are not canceled (`canceled_at` is empty and `status` is not `canceled`). The before snapshot of a clean run has no orders, so these are the run's orders.
- **Scenario carts**: the Medusa cart ids in the history's `carts` field.
- **Complete calls** for a cart: race operations with step `complete_cart` for that scenario cart, with their outcome (`ok`, `failed` or `unknown`) as defined in [history-format.md](history-format.md).
- **Order linked to a cart**: an `order_cart` row in `cart_orders` that is not deleted.
- Quantities are compared exactly, as decimals.

## Inconclusive runs

Every check in a run is `inconclusive`, with the cause as its reason, when any of these hold:

1. The history has a `harness_error`.
2. The before snapshot or the after snapshot is missing.
3. The before snapshot is not a clean start:
   - it has any order,
   - it has any live reservation,
   - a level's `reserved_quantity` differs from its live reservation sum, or
   - a level's `stocked_quantity` differs from the expected stock. Expected stock is the stock in effect when the race starts: the scenario's `stock_before_race` value for that inventory item, else its `stock` value, else the seeded stock (`WIDGET` 10, `SOLO` 1). This choice of expected stock is part of the clean start, not of any check's rule, so no check's version changed with it.
4. The after snapshot is missing data the checks need: no levels, or no `variants` entry for a variant that a counted order line uses.

## no_oversell (version 1)

**Rule.** The run never promises more units than were in stock.

**Definition.** For each level in the after snapshot:

- (a) the live reservation sum is at most the level's `stocked_quantity` in the after snapshot.

For each inventory item:

- (b) the units all counted order lines need for that item, added together, are at most the `stocked_quantity` of that item in the before snapshot, summed over its locations.

Medusa's `reserved_quantity` counter is never used by this check. It reads only orders, reservations and stock.

**Failure evidence.** For each inventory item and location: stocked before, stocked after, live reservation sum, and units needed by counted orders.

**Inconclusive.** Only in the cases listed under [Inconclusive runs](#inconclusive-runs).

## reserved_matches_reservations (version 1)

**Rule.** Medusa's reserved counter agrees with its own reservation rows.

**Definition.** For each level in the after snapshot, `reserved_quantity` equals the live reservation sum.

**Failure evidence.** For each inventory item and location: `reserved_quantity` and the live reservation sum.

**Inconclusive.** Only in the cases listed under [Inconclusive runs](#inconclusive-runs).

## one_order_per_cart (version 1)

**Rule.** One cart becomes at most one order, and everyone who completed it was told about the same order.

**Definition.**

- (a) No cart id is linked to more than one distinct order in the after snapshot.
- (b) For each scenario cart, every complete call with outcome `ok` returned the same order id (`response_body.order.id`).

**Failure evidence.** For (a): the cart id and its order ids. For (b): the scenario cart, each `ok` call's action id and operation id, and the order id it returned.

**Inconclusive.** Only in the cases listed under [Inconclusive runs](#inconclusive-runs).

## no_orphaned_orders (version 1)

**Rule.** Every order comes from a scenario cart and holds exactly the reservations its lines need, and a cart whose completion definitely failed has no order.

**Definition.**

- (a) Every counted order is linked to a cart, and that cart is a scenario cart.
- (b) For every line of every counted order and every inventory item the line needs, the live reservations tied to that line add up to exactly the units it needs. A live reservation is tied to an order line when its `line_item_id` is the order line's `line_item_id`, or when its `line_item_id` is a line of the order's linked cart (`cart_lines`) for the same variant. Medusa 2.21.2 ties reservations to order lines directly. Only reservations for the same inventory item count.
- (c) For each scenario cart that had at least one complete call, if every one of its complete calls has outcome `failed`, no order is linked to that cart. A complete call with outcome `unknown` means the server may have committed, so a cart with any `unknown` call is skipped for this part. Unknown calls never make this part pass or fail.

**Failure evidence.** For (a): the order id and its linked cart id, if any. For (b): the order id, line id, variant SKU, inventory item, units needed and units reserved. For (c): the scenario cart, its complete calls with their outcomes, and the linked order ids.

**Inconclusive.** Only in the cases listed under [Inconclusive runs](#inconclusive-runs).

## Verdicts and exit codes

Each history gets a `verdicts` list with one entry per chosen check: name, version, result, a one-line reason and the evidence numbers.

`oversold run` and `oversold check` exit with:

| Code | Meaning |
|---|---|
| 0 | Every chosen check passed in every run. |
| 1 | At least one check failed in at least one run. |
| 2 | Invalid scenario or bad usage. |
| 3 | Nothing failed, but at least one run was inconclusive, or the harness could not start. |

## Version history

- no_oversell 1: first version.
- reserved_matches_reservations 1: first version.
- one_order_per_cart 1: first version.
- no_orphaned_orders 1: first version.
