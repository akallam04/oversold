# Scenario format v1

A scenario is one YAML file that says which engine to start, how to set up stock and carts, which requests to race, and how many times to repeat the run. `oversold run` executes it and checks every run. `oversold validate` checks the file without running anything.

```sh
./bin/oversold validate scenarios/medusa/duplicate-complete.yaml
./bin/oversold run scenarios/medusa/duplicate-complete.yaml --runs 10
./bin/oversold run scenarios/medusa/duplicate-complete.yaml --runs 3 --version 2.19.0
./bin/oversold run scenarios/medusa/two-carts-shared-stock.yaml --runs 3 --version 2.22.0 --variant pr-16961
```

## Decoding rules

- The file holds exactly one YAML document.
- Decoding is strict. A field that is not listed below is an error, at any level.
- `oversold validate` prints every problem it finds, one per line, and exits 2 if there is at least one. It exits 0 when the file is valid.
- `oversold run` validates first and exits 2 on an invalid file without touching Docker. It also exits 2 for `--runs` below 1 or above 1000, and for a `--variant` that does not exist or is built on another version than the one the run uses.

## Fields

| Field | Type | Required | Rules |
|---|---|---|---|
| `schema` | string | yes | Must be `oversold.scenario/v1`. |
| `name` | string | yes | Kebab-case: lowercase letters, digits and single dashes, at most 64 characters. Used in the output folder name. |
| `description` | string | yes | Plain text. Not empty. |
| `target.engine` | string | yes | Only `medusa`. |
| `target.version` | string | yes | A Medusa version Oversold builds an image for: one with a folder under `targets/medusa/versions/` whose `package.json` pins every `@medusajs` package to exactly that version, with a matching `package-lock.json`. In this repository those are `2.19.0`, `2.21.2` and `2.22.0`. Each version has its own image (`oversold-medusa:<version>` by default, see `OVERSOLD_IMAGE_PREFIX` in the README) and its own template database. `oversold run --version X` runs the file against version X instead, and the history and report record both. |
| `target.servers` | integer | yes | `1` or `2`. With 2, a second Medusa server (`medusa-2` on 127.0.0.1:9001 by default, `MEDUSA_2_PORT`) runs on the same database. |
| `target.variant` | string | no | A build variant under `targets/medusa/variants/`, see [variant-format.md](variant-format.md). Its `base_version` must equal `target.version`. `oversold run --variant X` runs the file with variant X instead, and the version the run uses must then be X's base version. Without a variant, the run uses the plain image for its version. The field with no value is an error. |
| `target.locking` | string | no | `in-memory` or `redis`. Left out, it is `in-memory`: Medusa's default in-memory locking provider, which only locks inside one server process. With `redis`, every Medusa server in the run uses Medusa's Redis locking provider (`@medusajs/medusa/locking-redis`) as the default Locking Module provider, against the `redis` service in `compose.yaml`. Medusa recommends that provider when more than one server runs. The event bus, cache and workflow engine stay in-memory either way. An empty value, or the field with no value, is an error. |
| `stock` | map | no | Inventory SKU to stocked quantity, from 0 to 1000000. Keys are `WIDGET` or `SOLO`. Applied after the reset through the Admin API. Items not listed keep their seeded stock. |
| `stock_before_race` | map | no | Inventory SKU to stocked quantity, from 0 to 1000000, with the same keys as `stock`. Applied through the Admin API after every cart is prepared and before the before-race snapshot, so a cart can be built against one stock level and completed against another. An empty map, or the field with no value, is an error. When it is set, the clean start in [checks.md](checks.md) expects these values for the SKUs it lists. |
| `carts` | map | yes | Cart name (kebab-case) to a cart. At least one cart. |
| `carts.<name>.server` | integer | yes | The server the cart is built on, from 1 to `target.servers`. |
| `carts.<name>.count` | integer | no | From 1 to 100, default 1. With a count of N above 1, the scenario gets N identical carts named `<name>-1` to `<name>-N`, all on the cart's server. With 1, the cart keeps its name. |
| `carts.<name>.lines` | list | yes | At least one line. |
| `carts.<name>.lines[].sku` | string | yes | A variant SKU from the catalog below. A SKU may appear once per cart. |
| `carts.<name>.lines[].qty` | integer | yes | From 1 to 1000. |
| `race` | list | yes | At least one action. |
| `race[].id` | string | yes | Kebab-case, unique in the scenario. |
| `race[].server` | integer | yes | The server the action is sent to, from 1 to `target.servers`. |
| `race[].do` | string | yes | Only `complete` for now. Sends `POST /store/carts/{id}/complete`. |
| `race[].cart` | string | yes | A cart name from `carts`. Two actions may name the same cart. An action on a cart with a count of N above 1 becomes N actions, ids `<id>-1` to `<id>-N`, one per copy, released with the rest. |
| `timing.release` | string | yes | `together`: every race action waits on one barrier and all are released at once. `sequential`: actions run one after another in the listed order, each waiting for the previous response. |
| `timing.jitter_ms` | [min, max] | no | Extra delay before each race action, drawn from a seeded random source. `0 <= min <= max <= 10000`. Default: no delay. |
| `timing.request_timeout_ms` | integer | no | Timeout for every HTTP request in the run, from 100 to 120000. Default 30000. |
| `faults` | list | no | Must be empty or absent. Fault kinds come later. |
| `runs` | integer | yes | From 1 to 1000. `oversold run --runs N` overrides it. |
| `seed` | integer | yes | Seeds the jitter. The same seed and run index always give the same delays. |
| `checks` | list | no | Check names from the fixed list below, each at most once. Leave the field out to run all four. An empty list, or the field with no value, is an error. |

Carts are prepared in the order they appear in the file, copies of a counted cart in number order. After expanding counts, cart names and action ids must still be unique, and a scenario may have at most 200 race actions.

## Catalog

The seed creates a fixed catalog. Scenarios can only use these SKUs.

| Variant SKU | Inventory item | Units of the item per unit ordered | Seeded stock |
|---|---|---|---|
| `OVS-PIECE` | `WIDGET` | 1 | 10, shared |
| `OVS-TWOPACK` | `WIDGET` | 2 | 10, shared |
| `OVS-SOLO` | `SOLO` | 1 | 1 |

## Checks

A scenario chooses which checks apply to it. Checks take no parameters, so a scenario can pick them but never change how they work. [checks.md](checks.md) has each rule's exact definition and version.

| Name | Meaning |
|---|---|
| `no_oversell` | Live reservations never exceed stock, and the run's orders never need more units than were stocked at the start of the race. |
| `reserved_matches_reservations` | The reserved quantity on each inventory level equals the sum of its live reservations. |
| `one_order_per_cart` | A cart is linked to at most one order, and every `ok` complete call for a cart returned the same order. |
| `no_orphaned_orders` | Every order comes from a scenario cart and holds the reservations its lines need, and a cart whose completion definitely failed has no order. |

## What one run does

1. Make sure the Medusa image for the run set exists and was built from the current inputs, see "Images" below. Stop the Medusa servers. For a `redis` scenario, start Redis if it is not running and empty it with `FLUSHALL`, so no lock key carries over from the run before.
2. Drop the `medusa` database and create it again from the template database for the target version (for example `medusa_template_2_21_2`). If the template is missing, or was built from a different image, Oversold builds it first by running `medusa db:migrate` and the seed once. Migrate and seed always use the default in-memory config, so one template serves both locking modes.
3. Start the servers the scenario needs and wait for `GET /health` on each. The restart clears Medusa's in-memory locks, cache and workflow state.
4. Set stock through the Admin API.
5. Prepare each cart on its server: lines, email, US address, shipping method, payment collection and a `pp_system_default` payment session.
   If the scenario sets `stock_before_race`, set that stock through the Admin API once every cart is prepared.
6. Take a database snapshot. For a `redis` scenario, also read Redis's `INFO commandstats`.
7. Open one HTTP connection per race action with a `GET /health` warm-up, then release the actions.
8. Wait 2 seconds. This settle time is fixed in code.
9. For a `redis` scenario, read `INFO commandstats` again and count the `EVAL` and `EVALSHA` calls made since step 6. Medusa's Redis provider takes, renews and releases every lock with a Lua script, so this is the number of scripted lock calls the servers made during the race. A count of zero means the servers did not lock through Redis, and the run is a harness error.
10. Take a second database snapshot, compute the verdict of every chosen check, and write the history.

Before the race, Oversold also reads each server's image inputs label and, for a variant, the SHA-256 of every replaced file inside each running container. A label that differs from the run set's inputs, or a replaced file that differs from `variant.yaml`, is a harness error.

Any setup call that does not end `ok` is a harness error. That run is written with every check `inconclusive`, and the next run starts. `oversold run` stops early only after 3 harness errors in a row, and the report says so.

After the last run, `oversold run` writes `report.json` and `report.md` in the run folder and prints the report path last. `oversold check <run folder>` recomputes every verdict and the report from the saved histories, with no Docker.

Exit codes:

| Code | Meaning |
|---|---|
| 0 | Every chosen check passed in every run. |
| 1 | At least one check failed in at least one run. |
| 2 | Invalid scenario or bad usage. |
| 3 | Nothing failed, but at least one run was inconclusive, or the harness could not start. |

## Images

Each version has one image, `<prefix>:<version>`, and each variant one more, `<prefix>:<version>-<variant>`. Oversold labels every image it builds with `oversold.image-inputs`: the SHA-256 of a list of every file the image is built from, one `<sha256>  <path>` line per file sorted by path, with paths relative to `targets/medusa/`. The files are `Dockerfile`, `.dockerignore`, everything under `app/`, everything under `versions/<version>/`, and for a variant everything under `variants/<variant>/`, leaving out what `.dockerignore` leaves out (`node_modules`, `.medusa` and `.env`). Before a run set uses an image, Oversold computes that hash again and rebuilds the image if its label is missing or different. A rebuilt image also gets a new template database, since each template records the image it was built from. Images built by `make images` or `make up` have an empty label, so Oversold rebuilds them once before its first run on them, which mostly reuses Docker's layer cache.

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
