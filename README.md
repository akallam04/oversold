# Oversold

[![ci](https://github.com/akallam04/oversold/actions/workflows/ci.yml/badge.svg)](https://github.com/akallam04/oversold/actions/workflows/ci.yml) [![repro](https://github.com/akallam04/oversold/actions/workflows/repro.yml/badge.svg)](https://github.com/akallam04/oversold/actions/workflows/repro.yml)

**Oversold tests whether an online store can sell more than it has in stock.**

Picture a shop with one item left. Two people click "Place order" at the same moment. The shop should sell it once and tell the other person it's sold out. If both get "Order confirmed", the shop has sold something it doesn't have, and one of those orders can't be filled. That's an oversell.

Bugs like this are easy to miss, because they only show up when two things happen at almost exactly the same time. Oversold makes that happen on purpose:

1. It runs a real online store on one computer. The store software is [Medusa](https://github.com/medusajs/medusa), which is free and open source.
2. It has test shoppers check out at the same moment, within a fraction of a millisecond of each other.
3. It looks inside the store's database to see what really happened, and saves a full record of every test so anyone can check the result.

## What it showed

Tested on three Medusa releases from 2026 (2.19.0, 2.21.2 and 2.22.0):

- **The last item, sold twice.** When the store ran on two servers, as shops do to handle more visitors, both shoppers got the last item in 87 of 90 tries with Medusa's default setup. With the shared lock Medusa's docs recommend for more than one server, it happened in 0 of 90. ([#17032](https://github.com/medusajs/medusa/issues/17032), [#16569](https://github.com/medusajs/medusa/issues/16569))
- **The cart that didn't add up.** A cart with 4 single widgets and 2 two-packs needs 8 widgets. With only 6 in stock, the store accepted it in 30 of 30 tries on every release, because it checked each line of the cart on its own and never added them up. This one needs no rush: one shopper, one click. ([#16502](https://github.com/medusajs/medusa/issues/16502), [#17436](https://github.com/medusajs/medusa/issues/17436))
- **Two proposed fixes, tested.** Other developers proposed a fix for each bug. Oversold built the store with each fix and ran the same tests. Each fix stopped its own bug, and neither stopped both:

| Medusa 2.22.0, 30 tries each | Last item sold twice | Cart of 8 accepted with 6 in stock |
|---|---|---|
| As released | 30 of 30 | 30 of 30 |
| With fix [#17420](https://github.com/medusajs/medusa/pull/17420) | 0 of 30 | 30 of 30 |
| With fix [#16961](https://github.com/medusajs/medusa/pull/16961) | 30 of 30 | 0 of 30 |

**[See one real test, step by step](docs/walkthrough.md):** what each shopper sent, what the store answered, and what was in the database afterwards.

The bugs were reported by other people on Medusa's public issue tracker, and the two fixes were proposed by other developers. Oversold turned each report into a repeatable test, measured how often each bug happens, and checked both fixes.

## In numbers

- 3,120 recorded test runs, each saved in full in [results/](results/README.md).
- Every saved result is rechecked automatically on every change.
- It only tests copies running on the local machine, never real stores.

---

## For engineers

Oversold is a Go test harness that applies Jepsen-style history checking to full checkout flows. It races concurrent checkouts against local Docker copies of Medusa, records every request with its outcome (`ok`, `failed` or `unknown`), reads the order, reservation and stock rows before and after the race in read-only Postgres snapshots, and checks stock and order rules over that saved history. Built with Go 1.27, Docker Compose, PostgreSQL 15.19 and Redis 7.4.11. GitHub Actions rechecks every saved result on every push.

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
| last-unit-two-servers-redis-locking | control for #17032 with Redis locking | every check passes in every run |
| counter-two-servers | [#17032](https://github.com/medusajs/medusa/issues/17032) | `reserved_matches_reservations` fails in at least one run, the other checks pass in every run |
| counter-one-server | control for #17032 | every check passes in every run |
| counter-two-servers-redis-locking | control for #17032 with Redis locking | every check passes in every run |

Exit 0, or a green repro badge, means the known bugs still reproduced and the controls still passed on that version. Exit 1 means an expectation was not met, and 3 means a run was inconclusive or the harness failed, so the counts cannot be trusted. Make itself exits 2 whenever the command fails, and its last line, `make: *** [repro] Error N`, shows the code from `oversold expect`. The table, `summary.md` and every run folder are in `runs/<UTC time>-expect-<version>/`. [docs/expect-format.md](docs/expect-format.md) describes the file.

Measured on one Apple Silicon Mac with Docker Desktop: the first `make repro` for a version built its Medusa image in 41 seconds and its template database in 7 seconds, and took 2 minutes 36 seconds in all. With the image already built, it took 1 minute 58 seconds, including the template. On a fresh machine, pulling the Node and Postgres base images adds to the first run.

## How it works

1. Start a pinned engine version and its database in Docker Compose.
2. Run scripted clients that race checkout completions against each other, on one Medusa server or two.
3. Record a history of every request with its start and end time, request, response and outcome: ok, failed, or unknown when it timed out.
4. Read the real order, reservation and stock rows and check stock and order rules.

## Checks

Checks are pure functions over one saved history, so they need no Docker or database. Each has a version, and [docs/checks.md](docs/checks.md) records every change to a rule. A saved history is always rechecked with the versions it recorded.

- `no_oversell`: live reservations never exceed stock, and the run's orders never need more units than were stocked at the start of the race. Judged from order, reservation and stock rows, never from Medusa's reserved counter.
- `reserved_matches_reservations`: Medusa's reserved counter equals the sum of its live reservation rows.
- `one_order_per_cart`: a cart becomes at most one order, and every successful complete call for it returned the same order.
- `no_orphaned_orders`: every order comes from a scenario cart and holds the reservations its lines need, and a cart whose completion definitely failed has no order.
- `no_missing_orders`: every successful complete call returned the id of exactly one saved order that is not canceled and is linked to that cart.

A run is `inconclusive` for every check when the harness failed, a snapshot is missing, or the run did not start clean. The newest check versions are also inconclusive when the after snapshot lost an inventory level, holds a reservation without a level, or has a level quantity that cannot be read.

[Known limits](docs/checks.md#known-limits) lists what the checks do not show.

## Results

[results/](results/README.md) holds 3,120 runs: 600 on each of 2.19.0, 2.21.2 and 2.22.0, 450 in the plain 2.22.0 rerun, 420 on pr-16961 and 450 on pr-17420. Each folder holds one scenario's 30 runs, with the scenario file, the report and every run's history. Every scenario in `scenarios/medusa/` ran on each of the three versions, pr-16961 ran 14 of them and pr-17420 ran 15, and the plain rerun repeated those 15 on the Medusa 2.22.0 image that pr-17420 was built on.

CI rechecks every folder with `oversold check --verify` on every push, and anyone can run the same check without Docker ([How to recheck](results/README.md#how-to-recheck)). The repro workflow reruns the known bugs next to their controls on every code change, on the newest version folder, and every Monday on the newest `@medusajs/medusa` release on npm.

- [Findings in detail](results/README.md#findings-in-detail): what the runs showed, with the counts behind each finding.
- [Bug scenarios side by side](results/README.md#bug-scenarios-side-by-side): each bug scenario on plain 2.22.0, with Redis locking and with each proposed fix.

## Targets

- Medusa 2.19.0 (released Aug 13, 2026), 2.21.2 (released Sep 28, 2026) and 2.22.0 (released Oct 21, 2026).
- One or two Medusa servers on one Postgres database.
- Medusa's default in-memory locking, or its Redis locking provider, chosen per scenario.
- Two build variants, each Medusa 2.22.0 with a community fix: pr-16961 ([medusajs/medusa#16961](https://github.com/medusajs/medusa/pull/16961) by pqa-labs) and pr-17420 ([medusajs/medusa#17420](https://github.com/medusajs/medusa/pull/17420) by haranishi).

The seed creates one US region, one stock location and these SKUs:

| SKU | Inventory item | Units per item | Stock |
|---|---|---|---|
| OVS-PIECE | WIDGET | 1 | 10 shared |
| OVS-TWOPACK | WIDGET | 2 | 10 shared |
| OVS-SOLO | SOLO | 1 | 1 |

[docs/targets.md](docs/targets.md) has the full list: how each image is built and pinned, how build variants are made from source, and how Medusa runs.

## Scenarios and histories

A scenario is a YAML file that sets stock, prepares carts and races requests against them, then repeats that for a number of runs. Every run starts from a fresh copy of a template database made right after migrate and seed, with the Medusa servers restarted, so runs cannot affect each other. Each run writes one JSON history with every HTTP call, its outcome (`ok`, `failed` or `unknown`), database snapshots before and after the race, and the verdict of each chosen check. Secrets are masked before a history is written.

- [docs/scenario-format.md](docs/scenario-format.md): every scenario field and rule, with an example.
- [docs/expect-format.md](docs/expect-format.md): the expectations file that `oversold expect` and `make repro` run.
- [docs/variant-format.md](docs/variant-format.md): the build variant folder, `variant.yaml`, and how its files are made and checked.
- [docs/history-format.md](docs/history-format.md): every history field, the outcome rules and what gets masked.
- [docs/checks.md](docs/checks.md): the exact rule behind each check, its version, and when a verdict is inconclusive.
- [docs/targets.md](docs/targets.md): the Medusa versions, images, build variants and settings Oversold runs against, and the seeded SKUs.
- [docs/walkthrough.md](docs/walkthrough.md): two real saved runs, step by step, with the files they came from.
- [scenarios/medusa/](scenarios/medusa/): the scenarios that ship with Oversold.

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

`make images` builds one image per version under `targets/medusa/versions/`. Both it and `make up` label the images they build with `oversold.image-inputs`, the same hash `oversold run` computes and `oversold image-inputs` prints, so `oversold run` uses those images without rebuilding them. `make up` builds the image for `MEDUSA_VERSION` in `.env` if needed, starts Postgres, runs `medusa db:migrate` once, seeds the store, starts Medusa on 127.0.0.1:9000 (`MEDUSA_PORT`) and waits until `GET /health` passes. `make down` removes the containers and all data, so every `make up` starts from a clean seed.

```sh
make up
./bin/oversold validate scenarios/medusa/normal-checkout.yaml
./bin/oversold run scenarios/medusa/normal-checkout.yaml
./bin/oversold run scenarios/medusa/duplicate-complete.yaml --runs 10
./bin/oversold run scenarios/medusa/last-unit-two-servers.yaml --runs 3 --version 2.19.0
./bin/oversold run scenarios/medusa/two-carts-shared-stock.yaml --runs 3 --version 2.22.0 --variant pr-16961
./bin/oversold check runs/<folder>
./bin/oversold check --verify runs/<folder>
./bin/oversold export runs/<folder> <dest>
./bin/oversold versions
./bin/oversold variant pr-16961
./bin/oversold image-inputs --version 2.22.0 --variant pr-16961
./bin/oversold expect expectations/medusa.yaml --version 2.21.2 --runs 5
make repro
scripts/build-variant.sh pr-16961
make images
make psql DB=medusa
make logs
make down
```

`oversold run` prints one line per run, each check's counts, and last the path of the report it writes in `runs/<UTC time>-<scenario name>-<version>/`, with `-<variant>` after the version for a variant. It builds the Medusa image and template database for a version the first time that version runs, and again whenever the files the image is built from change. `--runs` must be from 1 to 1000, in `oversold run` and `oversold expect` alike. It refuses to run if the Docker endpoint is not local, and refuses any Medusa or database URL whose host is not localhost, 127.0.0.1 or ::1.

`oversold check runs/<folder>` recomputes every verdict and the report from the saved histories alone, with the check versions each history recorded and no Docker. It reads plain and gzipped histories. `oversold check --verify runs/<folder>` writes nothing and exits 0 only when the stored verdicts and the report match the recomputed ones, and 1 when anything differs, whatever the verdicts are. `oversold export runs/<folder> <dest>` copies the scenario, the report and every history, gzipped, into an empty folder that `oversold check` can read the same way.

Both commands exit 0 when every chosen check passed in every run, 1 when any check failed, 2 for an invalid scenario or bad usage, and 3 when nothing failed but a run was inconclusive.

`oversold versions` prints the Medusa versions Oversold can run, oldest first. `oversold variant` lists the build variants, and `oversold variant <name>` checks one and prints its fields. `oversold image-inputs` prints the `oversold.image-inputs` label value for one image, for `--version X` or `MEDUSA_VERSION` and an optional `--variant NAME`, and with `--files` the list of files it is the SHA-256 of. `oversold expect <file>` runs every case of an expectations file on one version, `--version X` or the newest, and exits 0 when every expectation was met, 1 when one was not, 2 for an invalid file or bad usage, and 3 when a run was inconclusive or the harness failed.

To add a Medusa version, run `scripts/add-medusa-version.sh X.Y.Z` and commit the folder it creates. The script copies the newest folder under `targets/medusa/versions/`, sets every `@medusajs` package to exactly X.Y.Z, copies `engines.node` from `@medusajs/medusa@X.Y.Z`, stops if the Node version in `targets/medusa/Dockerfile` does not satisfy it, and makes the lockfile inside that Node image. It needs Docker, Go and network access to npm.

## Prior work

Jepsen (https://jepsen.io) records every operation it sends to a system, including ones whose outcome is unknown, and checks that history against the guarantees the system claims. Oversold applies that idea to full checkout flows. ACIDRain (Warszawski and Bailis, SIGMOD 2017, https://www.bailis.org/papers/acidrain-sigmod2017.pdf) showed that concurrent requests to self-hosted e-commerce applications could corrupt inventory, overspend gift cards and steal items.

## Safety

Oversold only runs against local copies it starts in Docker, never live stores or public services. Security bugs found in an engine are reported privately to that project first.

## Project layout

```
cmd/oversold/                  CLI entry point: validate, run, check, export, expect, versions, variant, image-inputs
internal/scenario/             scenario format v1, strict decoding and validation
internal/expect/               expectation format v1, oversold expect summaries and exit codes
internal/runner/               runs a scenario: reset, setup, barrier race, snapshots, verdicts
internal/checks/               the checks and every version of their rules, pure functions over one history
internal/report/               report.json and report.md, exit codes
internal/history/              history format v1, outcome rules and masking
internal/stack/                Docker Compose control, Docker names and ports, template database reset, target facts
internal/targets/              reads and checks the pinned versions and variants under targets/, image input hashes
internal/medusa/               Store API and Admin API client
internal/dbstate/              read-only Postgres snapshot of stock, reservations and orders
internal/localguard/           refuses URLs that are not localhost
internal/envfile/              reads .env
scenarios/medusa/              shipped scenarios
expectations/                  known bugs next to their controls, for oversold expect and make repro
results/                       measured run sets: scenario, report and gzipped histories
docs/                          scenario, expectation, variant and history formats, check rules, targets and a walkthrough of two saved runs
targets/medusa/Dockerfile      builds and runs one pinned Medusa version, and its variants
targets/medusa/app/            shared medusa-config.ts, tsconfig and seed script
targets/medusa/versions/       one pinned package.json and lockfile per Medusa version
targets/medusa/variants/       one folder per build variant: variant.yaml and the compiled files it replaces
targets/medusa/variants/NOTICE Medusa's copyright and license notice for the compiled files in every variant
targets/medusa/db/             creates the medusa database in Postgres
scripts/check-tools.sh         checks that required tools are installed
scripts/add-medusa-version.sh  adds a pinned Medusa version folder with its lockfile
scripts/build-variant.sh       rebuilds a variant's files from its pull request and checks them
scripts/variant-compile.js     compiles Medusa packages inside the Medusa image, used by build-variant.sh
.githooks/                     pre-commit and commit-msg hooks
.github/workflows/             ci: vet, test, build, compose check, secret scan, results verify. repro: known bugs and controls
compose.yaml                   Postgres, Medusa migrate, seed, two servers and Redis
.env.example                   template for .env
.gitleaks.toml                 gitleaks config
Makefile                       build, test, repro and stack commands
```

## License

MIT. See [LICENSE](LICENSE).

The compiled files under `targets/medusa/variants/*/files/` are copies of Medusa's files with a pull request's change applied. They are under Medusa's MIT license, and [targets/medusa/variants/NOTICE](targets/medusa/variants/NOTICE) holds Medusa's copyright and license notice for them.
