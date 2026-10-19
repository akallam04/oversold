# History format v1

`oversold run` writes one JSON file per run:

```
runs/<UTC start time>-<scenario name>-<medusa version used>/
  <scenario file>.yaml     exact copy of the scenario that was run
  run-001.json             history of run 1
  run-002.json             history of run 2
  ...
  report.json              verdict counts and every run's verdicts
  report.md                the same report as a table
```

Histories can also be stored gzipped as `run-NNN.json.gz`. `oversold check` reads both forms. `oversold export <run folder> <dest>` copies the scenario file, the report and every history, gzipped, into an empty `<dest>`, with the report's links pointing at the gzipped files. `oversold check` on the export gives the same verdicts.

A history records what Oversold sent, what came back, the database rows before and after the race, and the verdict of each chosen check. The verdicts are computed from the rest of the file alone. `oversold check <run folder>` recomputes them with the current rules and rewrites the report. It never changes a history.

## Secrets

Histories never hold secrets, so they are safe to commit and share. Oversold masks secret values as `[masked]` before a history is written:

- Request and response bodies: the values of keys such as `password`, `token`, `access_token`, `refresh_token`, `secret`, `api_key`, `authorization`, `cookie` and `redacted`, at any depth. The admin login call is recorded with its password and returned token masked.
- Anywhere in the file, including error text and snapshots: the exact values of the publishable API key, the admin token, the admin password, the Postgres password and Medusa's JWT and cookie secrets.
- Anywhere in the file: strings that look like Medusa API keys (`pk_` or `sk_` followed by 16 or more letters and digits) and JSON web tokens.

Request headers are not recorded, so the `x-publishable-api-key` and `Authorization` headers never reach a history. Snapshots read no column from Medusa's `api_key` table.

## Top level

| Field | Meaning |
|---|---|
| `schema` | Always `oversold.history/v1`. |
| `scenario.name` | Scenario name. |
| `scenario.schema` | Scenario schema, `oversold.scenario/v1`. |
| `scenario.file` | File name of the scenario copy in the same folder. |
| `scenario.sha256` | SHA-256 of the scenario file bytes. |
| `run.index` | Run number, starting at 1. |
| `run.runs` | Number of runs in this invocation. |
| `run.seed` | Scenario seed. Jitter for a run is drawn from `seed` and `index`. |
| `oversold.commit` | Git commit Oversold was built from, read from the Go build info. |
| `oversold.dirty` | `true` if the build had uncommitted changes. |
| `target` | Facts read from the running system, see below. |
| `start`, `end` | Wall-clock start and end of the run, UTC. |
| `reset` | Database reset timing, see below. |
| `stock` | Stock the scenario set through the Admin API. |
| `carts` | Scenario cart name to Medusa cart id. |
| `race` | How the race was released, see below. |
| `operations` | Every HTTP call in the run, in the order they started. |
| `before`, `after` | Database snapshots just before the race and 2 seconds after it ended. |
| `harness_error` | Set when Oversold itself failed. Every check in the run is then `inconclusive`. |
| `verdicts` | One entry per chosen check, see below. |

## Target

| Field | Meaning |
|---|---|
| `engine` | `medusa`. |
| `scenario_version` | The version in the scenario file. |
| `version` | The version this run used: the file's version, or the one given with `oversold run --version`. |
| `servers[].index` | Server number used in the scenario (1 or 2). |
| `servers[].service` | Compose service name (`medusa` or `medusa-2`). |
| `servers[].url` | Base URL Oversold used. Always on 127.0.0.1. |
| `servers[].container_id` | Docker container id after the reset. |
| `servers[].image_id` | Image id the container runs. |
| `servers[].started_at` | Container start time reported by Docker. The reset restarts the container, so this changes every run. |
| `servers[].medusa_version` | `@medusajs/medusa` version read from inside the container. |
| `postgres_version` | `server_version` reported by Postgres. |
| `template_database` | Template the `medusa` database was copied from. |

Oversold refuses to start a run if any server reports a Medusa version other than `scenario_version`.

## Reset

| Field | Meaning |
|---|---|
| `start`, `healthy` | Wall-clock time the reset began and the time every server answered `GET /health` with 200. |
| `start_ns`, `healthy_ns` | The same moments as monotonic offsets from the start of the run. |
| `template_created` | `true` if this run had to build the template first. |
| `image_built` | `true` if this run had to build the Medusa image for its version first. |

## Race

| Field | Meaning |
|---|---|
| `release` | `together` or `sequential`. |
| `release_ns` | Monotonic offset when the barrier opened. For `sequential`, when the first action was allowed to start. |
| `settle_ms` | Fixed wait between the race and the second snapshot, 2000. |
| `actions[].id`, `do`, `cart`, `server` | Copied from the scenario. |
| `actions[].jitter_ns` | Delay applied before the request was sent. |
| `actions[].operation_id` | The operation that carried this action. Missing if it was never sent. |
| `actions[].outcome` | Outcome of that operation. |

## Operations

| Field | Meaning |
|---|---|
| `id` | Number of the operation in the run, starting at 1, in the order calls started. |
| `phase` | `setup` or `race`. Warm-up calls on race connections are `setup`. |
| `step` | What the call does, for example `admin_login`, `set_stocked_quantity`, `create_cart`, `add_line_item`, `create_payment_session`, `warm_up`, `complete_cart`. |
| `action_id` | Race action the call belongs to, for its warm-up and its race call. |
| `cart` | Scenario cart name, when the call is about a cart. |
| `server` | Server number the call was sent to. |
| `method`, `path` | HTTP method, and path with query string. |
| `request_body` | JSON request body, masked. A body that is not JSON is stored as a JSON string. |
| `status` | HTTP status, or 0 when no response arrived. |
| `response_body` | JSON response body, masked. A body that is not JSON is stored as a JSON string. |
| `start`, `end` | Wall-clock time just before the request was sent and just after the response body was read, UTC with 9 fractional digits. The wall clock on macOS has microsecond resolution, so the last 3 digits are often zero there. |
| `start_ns`, `end_ns` | The same moments as monotonic offsets in nanoseconds from the start of the run. Use these to compare operations. |
| `outcome` | `ok`, `failed` or `unknown`, see below. |
| `error` | Transport error or HTTP error text, if any. |
| `timeout` | `true` if the call hit the request timeout. |
| `conn_reused` | `true` if the call went over a connection opened earlier. Race calls should show `true`, since each one reuses its warm-up connection. |

## Outcome rules

| Outcome | When |
|---|---|
| `ok` | The server definitely did it. For `complete_cart`: HTTP 200 with `type` `order` and an order id. For every other step: any 2xx. |
| `failed` | The server definitely did not do it: any 4xx, or for `complete_cart` an HTTP 200 with `type` `cart` and an `error`. |
| `unknown` | The server may or may not have committed: timeouts, connection errors, 5xx, other status codes, and a `complete_cart` 200 that matches neither rule above. |

Setup calls follow the same rules. Any setup call that is not `ok` makes the run a harness error.

## Snapshots

Both snapshots are read in one read-only, repeatable-read transaction from the `medusa` database. They cover the seeded variants `OVS-PIECE`, `OVS-TWOPACK` and `OVS-SOLO`.

| Field | Source tables | Meaning |
|---|---|---|
| `taken_at` | | Postgres `now()` for the snapshot transaction. |
| `variant_skus` | | SKUs the snapshot covers. |
| `variants[]` | `product_variant`, `product_variant_inventory_item`, `inventory_item` | Each variant's inventory items and `required_quantity`. |
| `inventory_levels[]` | `inventory_level`, `inventory_item` | Stocked and reserved quantity per item and location. |
| `live_reservations[]` | `reservation_item`, `order_line_item`, `cart_line_item` | Every reservation row that is not deleted, with the line item it belongs to. |
| `orders[]` | `order`, `order_item`, `order_line_item` | Orders holding a covered SKU, with their lines at the current order version. |
| `orders[].payment_collections[]` | `order_payment_collection`, `payment_collection` | Each order's payment collections with status and amount. |
| `cart_orders[]` | `order_cart` | Cart to order links for those orders. |
| `cart_lines[]` | `cart_line_item` | Lines of the carts linked to those orders, used to tie reservations made against cart lines. |

Quantities are written as JSON numbers exactly as Postgres returns them.

## Verdicts

| Field | Meaning |
|---|---|
| `check` | Check name. |
| `version` | Version of the check's rule, see [checks.md](checks.md). |
| `result` | `pass`, `fail` or `inconclusive`. |
| `reason` | One line saying why. |
| `evidence[].subject` | What the numbers are about, for example `WIDGET at sloc_...` or `cart main`. |
| `evidence[].numbers` | The numbers the check compared, for example `stocked_before`, `live_reservations` and `order_units`. |
| `evidence[].values` | Ids and outcomes behind the numbers. |

## Report

`report.json` and `report.md` hold the scenario name and file hash, the Medusa version in the file and the versions used, the Oversold commits that ran it and the commit that computed the verdicts, the Medusa version and image per server, the Postgres version, the number of runs planned and recorded, whether the run stopped early, each check's fail, pass and inconclusive counts as k of n, the exit code, and every run's verdicts with a link to its history file.
