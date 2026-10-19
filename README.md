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
make up
make psql
make down
```

## Safety

Oversold only runs against local copies it starts in Docker, never live stores or public services. Security bugs found in an engine are reported privately to that project first.

## Project layout

```
cmd/oversold/          CLI entry point and its tests
scripts/check-tools.sh checks that required tools are installed
.githooks/             pre-commit and commit-msg hooks
.github/workflows/     CI: vet, test, build, compose check, secret scan
compose.yaml           local Postgres for development
.env.example           template for .env
.gitleaks.toml         gitleaks config
Makefile               build, test and stack commands
```

## License

MIT. See [LICENSE](LICENSE).
