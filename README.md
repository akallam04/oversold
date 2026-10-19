# Oversold

[![ci](https://github.com/akallam04/oversold/actions/workflows/ci.yml/badge.svg)](https://github.com/akallam04/oversold/actions/workflows/ci.yml)

**Jepsen-style history checking applied to full checkout flows.**

Oversold is a test harness in Go that runs controlled concurrent checkouts against local copies of open-source commerce engines, injects failures, records every request and the real database rows, and checks money and stock rules against that history.

## Status

Early setup. There are no results yet. Every number added later comes with the raw run histories behind it.

## How it works

1. Start a pinned engine version and its database in Docker Compose.
2. Run scripted clients that race checkouts, with faults such as a lost response or a killed server.
3. Record a history of every request with its start and end time, request, response and outcome: ok, failed, or unknown when it timed out.
4. Read the real order, reservation and stock rows and check the rules.

## Targets

- Medusa 2.21.2, built from a committed npm lockfile and run in production mode (`medusa build`, then `medusa start` from `.medusa/server`) on `node:22.23.3-bookworm-slim`.
- Runs in local Docker only, on its own `medusa` database in the Compose Postgres.
- Uses Medusa's default in-memory modules: local event bus, in-memory cache, in-memory locking and in-memory workflow engine. No Redis.
- Admin dashboard is off. Only the Store API is used.

The seed creates one US region, one stock location and these SKUs:

| SKU | Inventory item | Units per item | Stock |
|---|---|---|---|
| OVS-PIECE | WIDGET | 1 | 10 shared |
| OVS-TWOPACK | WIDGET | 2 | 10 shared |
| OVS-SOLO | SOLO | 1 | 1 |

## Checks (planned)

- No oversell, judged from order, reservation and stock rows together, not just the stock counter.
- One order per cart.
- No orphaned orders.

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

In `.env`, replace each `change-me` secret with a random value, for example from `openssl rand -hex 32`.

`make up` builds the Medusa image if needed, starts Postgres, runs `medusa db:migrate` once, seeds the store, starts Medusa on 127.0.0.1:9000 and waits until `GET /health` passes. `make down` removes the containers and all data, so every `make up` starts from a clean seed.

```sh
make up
./bin/oversold checkout --lines OVS-PIECE:2,OVS-TWOPACK:1 --label normal
make psql DB=medusa
make logs
make down
```

`oversold checkout` takes a database snapshot, runs one full Store API checkout (cart, line items, email and address, shipping, payment collection, `pp_system_default` session, complete), takes a second snapshot and writes everything to `runs/<UTC time>-<label>.json`. The run file holds every HTTP call with its request, response, status, start and end time, plus the inventory levels, live reservations, orders and cart to order links before and after. It refuses any Medusa or database URL whose host is not localhost, 127.0.0.1 or ::1.

## Safety

Oversold only runs against local copies it starts in Docker, never live stores or public services. Security bugs found in an engine are reported privately to that project first.

## Project layout

```
cmd/oversold/                  CLI entry point, checkout command and tests
internal/medusa/               Store API client, call recorder and checkout flow
internal/dbstate/              read-only Postgres snapshot of stock, reservations and orders
internal/localguard/           refuses URLs that are not localhost
targets/medusa/Dockerfile      builds and runs one pinned Medusa version
targets/medusa/app/            shared medusa-config.ts, tsconfig and seed script
targets/medusa/versions/       one pinned package.json and lockfile per Medusa version
targets/medusa/db/             creates the medusa database in Postgres
scripts/check-tools.sh         checks that required tools are installed
.githooks/                     pre-commit and commit-msg hooks
.github/workflows/             CI: vet, test, build, compose check, secret scan
compose.yaml                   Postgres, Medusa migrate, seed and server
.env.example                   template for .env
.gitleaks.toml                 gitleaks config
Makefile                       build, test and stack commands
```

## License

MIT. See [LICENSE](LICENSE).
