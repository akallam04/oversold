# Oversold

[![ci](https://github.com/akallam04/oversold/actions/workflows/ci.yml/badge.svg)](https://github.com/akallam04/oversold/actions/workflows/ci.yml) [![repro](https://github.com/akallam04/oversold/actions/workflows/repro.yml/badge.svg)](https://github.com/akallam04/oversold/actions/workflows/repro.yml)

**Jepsen-style history checking applied to full checkout flows.**

Oversold is a test harness in Go that races concurrent checkouts against local copies of open-source commerce engines, records every request and the real database rows, and checks stock and order rules against that history.

## Status

The measured runs are in [results/](results/README.md): every scenario in `scenarios/medusa/`, 30 runs each, on Medusa 2.19.0 (released Aug 13, 2026), 2.21.2 (released Sep 28, 2026) and 2.22.0 (released Oct 21, 2026), with every run's history. Anyone can recompute every verdict from those histories with `oversold check --verify`, and CI does so on every push.

The repro workflow reruns the known bugs next to their controls on every code change, on the newest version folder, and every Monday on the newest `@medusajs/medusa` release on npm.

## Reproduce in one command

With Docker running, Go 1.27 installed and `.env` copied from `.env.example`:

```sh
make repro
make repro MEDUSA=2.21.2
```

`make repro` builds Oversold, starts only Postgres, and runs `./bin/oversold expect expectations/medusa.yaml` on the newest folder under `targets/medusa/versions/`, or on the version given with `MEDUSA=X`. Each case runs 3 times, each run on a fresh database:

| Case | Targets | Expected |
|---|---|---|
| single-cart-over-demand | [#16502](https://github.com/medusajs/medusa/issues/16502) | `no_oversell` fails in every run, the other checks pass in every run |
| normal-checkout | control for #16502 | every check passes in every run |
| last-unit-two-servers | [#17032](https://github.com/medusajs/medusa/issues/17032) | `no_oversell` and `reserved_matches_reservations` fail in at least one run, the other checks pass in every run |
| last-unit-one-server | control for #17032 | every check passes in every run |
| counter-two-servers | [#17032](https://github.com/medusajs/medusa/issues/17032) | `reserved_matches_reservations` fails in at least one run, the other checks pass in every run |
| counter-one-server | control for #17032 | every check passes in every run |

Exit 0, or a green repro badge, means the known bugs still reproduced and the controls still passed on that version. Exit 1 means an expectation was not met, and 3 means a run was inconclusive or the harness failed, so the counts cannot be trusted. Make itself exits 2 whenever the command fails, and its last line, `make: *** [repro] Error N`, shows the code from `oversold expect`. The table, `summary.md` and every run folder are in `runs/<UTC time>-expect-<version>/`. [docs/expect-format.md](docs/expect-format.md) describes the file.

Measured on one Apple Silicon Mac with Docker Desktop: the first `make repro` for a version built its Medusa image in 41 seconds and its template database in 7 seconds, and took 2 minutes 36 seconds in all. With the image already built, it took 1 minute 58 seconds, including the template. On a fresh machine, pulling the Node and Postgres base images adds to the first run.

## How it works

1. Start a pinned engine version and its database in Docker Compose.
2. Run scripted clients that race checkout completions against each other, on one Medusa server or two.
3. Record a history of every request with its start and end time, request, response and outcome: ok, failed, or unknown when it timed out.
4. Read the real order, reservation and stock rows and check stock and order rules.

## Targets

- Medusa 2.19.0 (released Aug 13, 2026), 2.21.2 (released Sep 28, 2026) and 2.22.0 (released Oct 21, 2026), each built from its own committed npm lockfile into its own image (`oversold-medusa:<version>` by default) and run in production mode (`medusa build`, then `medusa start` from `.medusa/server`) on `node:22.23.3-bookworm-slim`. Medusa 2.22.0 needs Node 22.22 or later. A scenario's `target.version` picks the image, and `oversold run --version X` overrides it.
- The versions Oversold can run are the folders under `targets/medusa/versions/`. A folder counts only when its `package.json` pins every `@medusajs` package to exactly that version and its `package-lock.json` matches. Adding a version needs no code change.
- Runs in local Docker only, on its own `medusa` database in the Compose Postgres. A second server, `medusa-2` on 127.0.0.1:9001 by default, shares the same database and starts only for scenarios that ask for two servers.
- Uses Medusa's default in-memory modules: local event bus, in-memory cache, in-memory locking and in-memory workflow engine. No Redis.
- Admin dashboard is off. Checkouts use the Store API. The Admin API is used only to set stock before a run.

The seed creates one US region, one stock location and these SKUs:

| SKU | Inventory item | Units per item | Stock |
|---|---|---|---|
| OVS-PIECE | WIDGET | 1 | 10 shared |
| OVS-TWOPACK | WIDGET | 2 | 10 shared |
| OVS-SOLO | SOLO | 1 | 1 |

## Scenarios and histories

A scenario is a YAML file that sets stock, prepares carts and races requests against them, then repeats that for a number of runs. Every run starts from a fresh copy of a template database made right after migrate and seed, with the Medusa servers restarted, so runs cannot affect each other. Each run writes one JSON history with every HTTP call, its outcome (`ok`, `failed` or `unknown`), database snapshots before and after the race, and the verdict of each chosen check. Secrets are masked before a history is written.

- [docs/scenario-format.md](docs/scenario-format.md): every scenario field and rule, with an example.
- [docs/expect-format.md](docs/expect-format.md): the expectations file that `oversold expect` and `make repro` run.
- [docs/history-format.md](docs/history-format.md): every history field, the outcome rules and what gets masked.
- [docs/checks.md](docs/checks.md): the exact rule behind each check, its version, and when a verdict is inconclusive.
- [scenarios/medusa/](scenarios/medusa/): the scenarios that ship with Oversold.

## Checks

Checks are pure functions over one saved history, so they need no Docker or database. Each has a version, and [docs/checks.md](docs/checks.md) records every change to a rule.

- `no_oversell`: live reservations never exceed stock, and the run's orders never need more units than were stocked at the start of the race. Judged from order, reservation and stock rows, never from Medusa's reserved counter.
- `reserved_matches_reservations`: Medusa's reserved counter equals the sum of its live reservation rows.
- `one_order_per_cart`: a cart becomes at most one order, and every successful complete call for it returned the same order.
- `no_orphaned_orders`: every order comes from a scenario cart and holds the reservations its lines need, and a cart whose completion definitely failed has no order.

A run is `inconclusive` for every check when the harness failed, a snapshot is missing, or the run did not start clean.

## Prior work

Jepsen (https://jepsen.io) records every operation it sends to a system, including ones whose outcome is unknown, and checks that history against the guarantees the system claims. Oversold applies that idea to full checkout flows. ACIDRain (Warszawski and Bailis, SIGMOD 2017, https://www.bailis.org/papers/acidrain-sigmod2017.pdf) showed that concurrent requests to self-hosted e-commerce applications could corrupt inventory, overspend gift cards and steal items.

## Run it

Requires Docker with Compose v2, Go 1.27 and gitleaks.

```sh
cp .env.example .env
git config core.hooksPath .githooks
make tools
make test
make build
```

In `.env`, replace each `change-me` value with a random one, for example from `openssl rand -hex 32`. `MEDUSA_ADMIN_EMAIL` and `MEDUSA_ADMIN_PASSWORD` are the local admin user the seed creates.

These `.env` keys set the Docker names and the local ports. Compose and `oversold run` read the same values, and a value set in the shell wins over `.env`.

| Key | Default | Sets |
|---|---|---|
| `COMPOSE_PROJECT_NAME` | `oversold` | Compose project, so the container, network and volume names |
| `OVERSOLD_IMAGE_PREFIX` | `oversold-medusa` | Medusa image name, `<prefix>:<version>` |
| `POSTGRES_PORT` | `55432` | Postgres on 127.0.0.1 |
| `MEDUSA_PORT` | `9000` | Medusa server 1 on 127.0.0.1 |
| `MEDUSA_2_PORT` | `9001` | Medusa server 2 on 127.0.0.1 |

To run a second copy of Oversold next to the first, give its `.env` a different project name, image prefix and ports. The two copies then share no containers, images, networks, volumes or ports.

`make images` builds one image per version under `targets/medusa/versions/`. `make up` builds the image for `MEDUSA_VERSION` in `.env` if needed, starts Postgres, runs `medusa db:migrate` once, seeds the store, starts Medusa on 127.0.0.1:9000 (`MEDUSA_PORT`) and waits until `GET /health` passes. `make down` removes the containers and all data, so every `make up` starts from a clean seed.

```sh
make up
./bin/oversold validate scenarios/medusa/normal-checkout.yaml
./bin/oversold run scenarios/medusa/normal-checkout.yaml
./bin/oversold run scenarios/medusa/duplicate-complete.yaml --runs 10
./bin/oversold run scenarios/medusa/last-unit-two-servers.yaml --runs 3 --version 2.19.0
./bin/oversold check runs/<folder>
./bin/oversold check --verify runs/<folder>
./bin/oversold export runs/<folder> <dest>
./bin/oversold versions
./bin/oversold expect expectations/medusa.yaml --version 2.21.2 --runs 5
make repro
make images
make psql DB=medusa
make logs
make down
```

`oversold run` prints one line per run, each check's counts, and last the path of the report it writes in `runs/<UTC time>-<scenario name>-<version>/`. It builds the Medusa image and template database for a version the first time that version runs. It refuses to run if the Docker endpoint is not local, and refuses any Medusa or database URL whose host is not localhost, 127.0.0.1 or ::1.

`oversold check runs/<folder>` recomputes every verdict and the report from the saved histories alone, with the current rules and no Docker. It reads plain and gzipped histories. `oversold check --verify runs/<folder>` writes nothing and exits 0 only when the stored verdicts and the report match the recomputed ones, and 1 when anything differs, whatever the verdicts are. `oversold export runs/<folder> <dest>` copies the scenario, the report and every history, gzipped, into an empty folder that `oversold check` can read the same way.

Both commands exit 0 when every chosen check passed in every run, 1 when any check failed, 2 for an invalid scenario or bad usage, and 3 when nothing failed but a run was inconclusive.

`oversold versions` prints the Medusa versions Oversold can run, oldest first. `oversold expect <file>` runs every case of an expectations file on one version, `--version X` or the newest, and exits 0 when every expectation was met, 1 when one was not, 2 for an invalid file or bad usage, and 3 when a run was inconclusive or the harness failed.

To add a Medusa version, run `scripts/add-medusa-version.sh X.Y.Z` and commit the folder it creates. The script copies the newest folder under `targets/medusa/versions/`, sets every `@medusajs` package to exactly X.Y.Z, copies `engines.node` from `@medusajs/medusa@X.Y.Z`, stops if the Node version in `targets/medusa/Dockerfile` does not satisfy it, and makes the lockfile inside that Node image. It needs Docker, Go and network access to npm.

## Safety

Oversold only runs against local copies it starts in Docker, never live stores or public services. Security bugs found in an engine are reported privately to that project first.

## Project layout

```
cmd/oversold/                  CLI entry point: validate, run, check, export, expect, versions
internal/scenario/             scenario format v1, strict decoding and validation
internal/expect/               expectation format v1, oversold expect summaries and exit codes
internal/runner/               runs a scenario: reset, setup, barrier race, snapshots, verdicts
internal/checks/               the four checks, pure functions over one history
internal/report/               report.json and report.md, exit codes
internal/history/              history format v1, outcome rules and masking
internal/stack/                Docker Compose control, Docker names and ports, template database reset, target facts
internal/targets/              reads and checks the pinned versions under targets/
internal/medusa/               Store API and Admin API client
internal/dbstate/              read-only Postgres snapshot of stock, reservations and orders
internal/localguard/           refuses URLs that are not localhost
internal/envfile/              reads .env
scenarios/medusa/              shipped scenarios
expectations/                  known bugs next to their controls, for oversold expect and make repro
results/                       measured run sets: scenario, report and gzipped histories
docs/                          scenario format, expectation format, history format and check rules
targets/medusa/Dockerfile      builds and runs one pinned Medusa version
targets/medusa/app/            shared medusa-config.ts, tsconfig and seed script
targets/medusa/versions/       one pinned package.json and lockfile per Medusa version
targets/medusa/db/             creates the medusa database in Postgres
scripts/check-tools.sh         checks that required tools are installed
scripts/add-medusa-version.sh  adds a pinned Medusa version folder with its lockfile
.githooks/                     pre-commit and commit-msg hooks
.github/workflows/             ci: vet, test, build, compose check, secret scan, results verify. repro: known bugs and controls
compose.yaml                   Postgres, Medusa migrate, seed and two servers
.env.example                   template for .env
.gitleaks.toml                 gitleaks config
Makefile                       build, test, repro and stack commands
```

## License

MIT. See [LICENSE](LICENSE).
