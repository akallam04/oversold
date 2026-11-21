# Checks

A check reads one run's history and gives a verdict: `pass`, `fail` or `inconclusive`. Checks are pure functions over the saved history. They need no Docker and no database, so anyone can recompute them later with `oversold check <run folder>`.

Each check has a name and a version. The version starts at 1. Any change to a rule bumps that check's version and adds a line to [Version history](#version-history) saying what changed and why. Every verdict records the check version that produced it, and older versions stay in the code, so a saved history is always rechecked with the versions it recorded. See [How old histories are verified](#how-old-histories-are-verified).

A scenario chooses its checks with the `checks` field. If the field is left out, every check applies: no_oversell, reserved_matches_reservations, one_order_per_cart, no_orphaned_orders and no_missing_orders.

What the checks do not show is listed under [Known limits](#known-limits).

## Terms

- **Before snapshot** and **after snapshot**: the `before` and `after` database snapshots in the history. The before snapshot is taken at the start of the race, after setup. The after snapshot is taken 2 seconds after the race ends.
- **Level**: one row of `inventory_levels`, identified by inventory item and location. It has `stocked_quantity` and `reserved_quantity`.
- **Live reservation**: one row of `live_reservations`, a `reservation_item` row that is not deleted.
- **Live reservation sum** of a level: the sum of `quantity` over live reservations with the same inventory item and location.
- **Units a line needs** for an inventory item: the line's `quantity` times the `required_quantity` for that variant and inventory item, taken from the snapshot's `variants`. A line can need units of more than one inventory item.
- **Counted orders**: orders in the after snapshot that are not canceled (`canceled_at` is empty and `status` is not `canceled`). The before snapshot of a clean run has no orders, so these are the run's orders.
- **Scenario carts**: the Medusa cart ids in the history's `carts` field.
- **Complete calls** for a cart: race operations with step `complete_cart` for that scenario cart, with their outcome (`ok`, `failed` or `unknown`) as defined in [history-format.md](history-format.md). A call is `ok` only when the server answered 2xx with an order and its id.
- **Order linked to a cart**: an `order_cart` row in `cart_orders` that is not deleted.
- Quantities are compared exactly, as decimals. A quantity that is empty or not a number (for example `null`) cannot be read.

## Inconclusive runs

Every check in a run is `inconclusive`, with the cause as its reason, when any of these hold:

1. The history has a `harness_error`.
2. The before snapshot or the after snapshot is missing.
3. The before snapshot is not a clean start:
   - it has any order,
   - it has any live reservation,
   - a level's `reserved_quantity` or `stocked_quantity` cannot be read,
   - a level's `reserved_quantity` differs from its live reservation sum, or
   - a level's `stocked_quantity` differs from the expected stock. Expected stock is the stock in effect when the race starts: the scenario's `stock_before_race` value for that inventory item, else its `stock` value, else the seeded stock (`WIDGET` 10, `SOLO` 1). This choice of expected stock is part of the clean start, not of any check's rule, so no check's version changed with it.
4. The after snapshot is missing data the checks need: no levels, no `variants` entry for a variant that a counted order line uses, or a `required_quantity`, live reservation `quantity` or counted order line `quantity` that cannot be read.
5. Only for no_oversell 2, reserved_matches_reservations 2, one_order_per_cart 2, no_orphaned_orders 2 and no_missing_orders 1, the after snapshot is missing rows or has a level quantity that cannot be read:
   - a level in the before snapshot has no level with the same inventory item and location in the after snapshot,
   - a live reservation has no level with its inventory item and location in the after snapshot, or
   - a level's `stocked_quantity` or `reserved_quantity` in the after snapshot cannot be read.

Version 1 of the first four checks does not have rule 5. It skips a level that is gone from the after snapshot and a reservation without a level. no_oversell 1 and reserved_matches_reservations 1 also cannot judge a history whose after level quantity cannot be read, so `oversold check` reports an error naming that history instead of a verdict.

## no_oversell (version 2)

**Rule.** The run never promises more units than were in stock.

**Definition.** For each level in the after snapshot:

- (a) the live reservation sum is at most the level's `stocked_quantity` in the after snapshot.

For each inventory item:

- (b) the units all counted order lines need for that item, added together, are at most the `stocked_quantity` of that item in the before snapshot, summed over its locations.

Medusa's `reserved_quantity` counter is never used by this check. It reads only orders, reservations and stock.

**Failure evidence.** For each inventory item and location: stocked before, stocked after, live reservation sum, and units needed by counted orders.

**Inconclusive.** Only in the cases listed under [Inconclusive runs](#inconclusive-runs), rules 1 to 5.

## reserved_matches_reservations (version 2)

**Rule.** Medusa's reserved counter agrees with its own reservation rows.

**Definition.** For each level in the after snapshot, `reserved_quantity` equals the live reservation sum.

**Failure evidence.** For each inventory item and location: `reserved_quantity` and the live reservation sum.

**Inconclusive.** Only in the cases listed under [Inconclusive runs](#inconclusive-runs), rules 1 to 5.

## one_order_per_cart (version 2)

**Rule.** One cart becomes at most one order, and everyone who completed it was told about the same order.

**Definition.**

- (a) No cart id is linked to more than one distinct order in the after snapshot.
- (b) For each scenario cart, every complete call with outcome `ok` returned the same order id (`response_body.order.id`).

**Failure evidence.** For (a): the cart id and its order ids. For (b): the scenario cart, each `ok` call's action id and operation id, and the order id it returned.

**Inconclusive.** Only in the cases listed under [Inconclusive runs](#inconclusive-runs), rules 1 to 5.

## no_orphaned_orders (version 2)

**Rule.** Every order comes from a scenario cart and holds exactly the reservations its lines need, and a cart whose completion definitely failed has no order.

**Definition.**

- (a) Every counted order is linked to a cart, and that cart is a scenario cart.
- (b) For every counted order and every inventory item its lines need, the live reservations tied to the order add up to exactly the units all its lines need for that item. A live reservation is tied to the order when its `line_item_id` is the `line_item_id` of one of the order's lines, or a line of the order's linked cart (`cart_lines`) for the same variant as one of the order's lines. Medusa 2.21.2 ties reservations to order lines directly. Only reservations for the same inventory item count, and each one counts once for the order, even when it is tied to more than one of the order's lines.
- (c) For each scenario cart that had at least one complete call, if every one of its complete calls has outcome `failed`, no order is linked to that cart. A complete call with outcome `unknown` means the server may have committed, so a cart with any `unknown` call is skipped for this part. Unknown calls never make this part pass or fail.

**Failure evidence.** For (a): the order id and its linked cart id, if any. For (b): the order id, inventory item, units needed and units reserved. For (c): the scenario cart, its complete calls with their outcomes, and the linked order ids.

**Inconclusive.** Only in the cases listed under [Inconclusive runs](#inconclusive-runs), rules 1 to 5.

## no_missing_orders (version 1)

**Rule.** Every completion the server confirmed left exactly one saved order for that cart.

**Definition.** For each scenario cart with at least one complete call, every complete call with outcome `ok` returned an order id (`response_body.order.id`) that:

- appears exactly once among the orders in the after snapshot,
- belongs to a counted order, so the order is not canceled, and
- is linked to that cart.

A cart with any complete call of outcome `unknown` is skipped, as in no_orphaned_orders (c). Calls with outcome `failed` are not judged here. no_orphaned_orders (c) covers them.

The other checks only look at orders that exist, so an `ok` call whose order is missing, canceled or linked to another cart passes all of them. This check closes that gap.

**Failure evidence.** For each scenario cart: its `ok` calls with the order id each returned, its linked order ids, and whether the cart was judged or skipped.

**Inconclusive.** Only in the cases listed under [Inconclusive runs](#inconclusive-runs), rules 1 to 5.

## Verdicts and exit codes

Each history gets a `verdicts` list with one entry per chosen check: name, version, result, a one-line reason and the evidence numbers. A report has one row for each check name and version found in its histories' verdicts.

`oversold run` and `oversold check` exit with:

| Code | Meaning |
|---|---|
| 0 | Every chosen check passed in every run. |
| 1 | At least one check failed in at least one run. |
| 2 | Invalid scenario or bad usage. |
| 3 | Nothing failed, but at least one run was inconclusive, or the harness could not start. |

## How old histories are verified

`oversold run` judges every new run with the newest version of each chosen check.

A saved history keeps the name and version of every check that judged it. `oversold check` and `oversold check --verify` recompute each stored verdict with that same name and version, so a history recorded with version 1 is still judged by version 1, and its report keeps its version 1 rows. A check that did not exist when the history was recorded, such as no_missing_orders for the histories in [results/](../results/README.md), is not added to it. Only a history with no stored verdicts gets the newest version of every check its scenario chooses.

`oversold check --verify` also requires the stored verdicts to name only checks the scenario chooses, each once and in the scenario's order.

## Known limits

- The checks look at one snapshot, taken 2 seconds after the last response. They don't show that a rule held at every moment, and they can miss a change that lands later.
- This is rule checking over recorded histories, not a linearizability checker.
- Only `complete` is raced, on one or two Medusa servers, with one stock location and no injected faults.
- The Redis lock-call count shows that lock scripts ran, not which keys they locked.
- `oversold check --verify` shows that the stored verdicts follow from the stored histories, not that the rules cover every business rule.
- A Medusa release that fixes a bug turns the repro badge red on purpose: the repro workflow expects each known bug to still fail, so a fix shows up as an unmet expectation.
- no_missing_orders skips every cart that has a complete call with an `unknown` outcome, including that cart's `ok` calls, and it passes when every cart is skipped. In the 3,120 published runs no complete call had an `unknown` outcome, so no published verdict depends on this.

## Version history

- no_oversell 1: first version.
- no_oversell 2: adds rule 5 under [Inconclusive runs](#inconclusive-runs). Version 1 skipped a level that was gone from the after snapshot, so it could pass a run it could not fully see, and an unreadable after level quantity stopped it with an error. The rule itself is unchanged.
- reserved_matches_reservations 1: first version.
- reserved_matches_reservations 2: adds rule 5, for the same reasons as no_oversell 2. The rule itself is unchanged.
- one_order_per_cart 1: first version.
- one_order_per_cart 2: adds rule 5, so that every check of a run uses the same preconditions. The rule itself is unchanged.
- no_orphaned_orders 1: first version.
- no_orphaned_orders 2: adds rule 5, and part (b) adds up reservations per order and inventory item, counting each live reservation once. Version 1 added them up per line, so a reservation tied through a cart line counted for every order line of that variant. With two lines of one variant, that could hide a missing unit or fail an order that held exactly what it needed.
- no_missing_orders 1: first version, with rule 5. Added because no check compared the `ok` complete calls with the orders saved in the after snapshot.
