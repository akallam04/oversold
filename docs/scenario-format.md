# Scenario format v1

A scenario is one YAML file that says which engine to start, how to set up stock and carts, which requests to race, and how many times to repeat the run. `oversold run` executes it. `oversold validate` checks it without running anything.

```sh
./bin/oversold validate scenarios/medusa/duplicate-complete.yaml
./bin/oversold run scenarios/medusa/duplicate-complete.yaml --runs 10
```

## Decoding rules

- The file holds exactly one YAML document.
- Decoding is strict. A field that is not listed below is an error, at any level.
- `oversold validate` prints every problem it finds, one per line, and exits 2 if there is at least one. It exits 0 when the file is valid.
- `oversold run` validates first and exits 2 on an invalid file without touching Docker.

## Fields

| Field | Type | Required | Rules |
|---|---|---|---|
| `schema` | string | yes | Must be `oversold.scenario/v1`. |
| `name` | string | yes | Kebab-case: lowercase letters, digits and single dashes, at most 64 characters. Used in the output folder name. |
| `description` | string | yes | Plain text. Not empty. |
| `target.engine` | string | yes | Only `medusa`. |
| `target.version` | string | yes | A Medusa version Oversold builds an image for. Only `2.21.2` for now. Must also match `MEDUSA_VERSION` in `.env`. |
| `target.servers` | integer | yes | `1` or `2`. With 2, a second Medusa server (`medusa-2` on 127.0.0.1:9001) runs on the same database. |
| `stock` | map | no | Inventory SKU to stocked quantity, from 0 to 1000000. Keys are `WIDGET` or `SOLO`. Applied after the reset through the Admin API. Items not listed keep their seeded stock. |
| `carts` | map | yes | Cart name (kebab-case) to a cart. At least one cart. |
| `carts.<name>.server` | integer | yes | The server the cart is built on, from 1 to `target.servers`. |
| `carts.<name>.lines` | list | yes | At least one line. |
| `carts.<name>.lines[].sku` | string | yes | A variant SKU from the catalog below. A SKU may appear once per cart. |
| `carts.<name>.lines[].qty` | integer | yes | From 1 to 1000. |
| `race` | list | yes | At least one action. |
| `race[].id` | string | yes | Kebab-case, unique in the scenario. |
| `race[].server` | integer | yes | The server the action is sent to, from 1 to `target.servers`. |
| `race[].do` | string | yes | Only `complete` for now. Sends `POST /store/carts/{id}/complete`. |
| `race[].cart` | string | yes | A cart name from `carts`. Two actions may name the same cart. |
| `timing.release` | string | yes | `together`: every race action waits on one barrier and all are released at once. `sequential`: actions run one after another in the listed order, each waiting for the previous response. |
| `timing.jitter_ms` | [min, max] | no | Extra delay before each race action, drawn from a seeded random source. `0 <= min <= max <= 10000`. Default: no delay. |
| `timing.request_timeout_ms` | integer | no | Timeout for every HTTP request in the run, from 100 to 120000. Default 30000. |
| `faults` | list | no | Must be empty or absent. Fault kinds come later. |
| `runs` | integer | yes | From 1 to 1000. `oversold run --runs N` overrides it. |
| `seed` | integer | yes | Seeds the jitter. The same seed and run index always give the same delays. |
| `checks` | list | no | Check names from the fixed list below, each at most once. |

Carts are prepared in the order they appear in the file.

## Catalog

The seed creates a fixed catalog. Scenarios can only use these SKUs.

| Variant SKU | Inventory item | Units of the item per unit ordered | Seeded stock |
|---|---|---|---|
| `OVS-PIECE` | `WIDGET` | 1 | 10, shared |
| `OVS-TWOPACK` | `WIDGET` | 2 | 10, shared |
| `OVS-SOLO` | `SOLO` | 1 | 1 |

## Checks

A scenario chooses which checks apply to it. Checks take no parameters, so a scenario can pick them but never change how they work. The names are validated now. The checks themselves are not built yet, and no run is judged.

| Name | Meaning |
|---|---|
| `no_oversell` | Units promised to orders never exceed stock, judged from order, reservation and stock rows together. |
| `reserved_matches_reservations` | The reserved quantity on each inventory level equals the sum of its live reservations. |
| `one_order_per_cart` | A cart is linked to at most one order. |
| `no_orphaned_orders` | Every order is linked to a cart and has its reservations. |

## What one run does

1. Stop the Medusa servers, drop the `medusa` database and create it again from the template database for the target version (for example `medusa_template_2_21_2`). If the template is missing, or was built from a different image, Oversold builds it first by running `medusa db:migrate` and the seed once.
2. Start the servers the scenario needs and wait for `GET /health` on each. The restart clears Medusa's in-memory locks, cache and workflow state.
3. Set stock through the Admin API.
4. Prepare each cart on its server: lines, email, US address, shipping method, payment collection and a `pp_system_default` payment session.
5. Take a database snapshot.
6. Open one HTTP connection per race action with a `GET /health` warm-up, then release the actions.
7. Wait 2 seconds. This settle time is fixed in code.
8. Take a second database snapshot and write the history.

Any setup call that does not end `ok` is a harness error. The history for that run is still written, and `oversold run` stops with exit code 3.

Exit codes: 0 when all runs finished, 2 for an invalid scenario or bad usage, 3 when the harness itself failed.

## Example

```yaml
schema: oversold.scenario/v1
name: last-unit-two-servers
description: >
  Two carts each hold the last SOLO unit. Both are completed at the same
  moment, one on each server.
target:
  engine: medusa
  version: 2.21.2
  servers: 2
stock:
  SOLO: 1
carts:
  alice:
    server: 1
    lines:
      - sku: OVS-SOLO
        qty: 1
  bob:
    server: 2
    lines:
      - sku: OVS-SOLO
        qty: 1
race:
  - id: alice-on-1
    server: 1
    do: complete
    cart: alice
  - id: bob-on-2
    server: 2
    do: complete
    cart: bob
timing:
  release: together
  jitter_ms: [0, 3]
  request_timeout_ms: 10000
faults: []
runs: 50
seed: 20261019
checks:
  - no_oversell
  - reserved_matches_reservations
  - one_order_per_cart
  - no_orphaned_orders
```
