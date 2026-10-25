# Expectation format v1

An expectations file lists scenarios and says, for every check each one runs, whether that check should pass in every run, fail in every run, or fail in at least one run. `oversold expect` runs every case through the normal runner on one Medusa version and compares the counts with the file. `make repro` runs [expectations/medusa.yaml](../expectations/medusa.yaml) this way.

```sh
./bin/oversold expect expectations/medusa.yaml
./bin/oversold expect expectations/medusa.yaml --version 2.21.2
./bin/oversold expect expectations/medusa.yaml --runs 10
./bin/oversold versions
```

## Decoding rules

- The file holds exactly one YAML document.
- Decoding is strict. A field that is not listed below is an error, at any level, and so is a check name given twice in one case.
- Every scenario a case names is loaded and validated with the same rules as `oversold validate`. A scenario that does not validate makes the file invalid.
- `oversold expect` prints every problem it finds, one per line, and exits 2 without touching Docker.

## Fields

| Field | Type | Required | Rules |
|---|---|---|---|
| `schema` | string | yes | Must be `oversold.expect/v1`. |
| `description` | string | no | Plain text. |
| `runs` | integer | yes | Runs per case, from 1 to 1000. It replaces each scenario's own `runs`. `oversold expect --runs N` overrides it. |
| `cases` | list | yes | At least one case. |
| `cases[].scenario` | string | yes | A scenario file, as a clean path relative to the repository root, for example `scenarios/medusa/normal-checkout.yaml`. No absolute paths and no `..`. Each scenario at most once per file. |
| `cases[].note` | string | no | One line of at most 80 characters, for example the issue the case targets. Shown in the summary. |
| `cases[].expect` | map | yes | Check name to expectation. It must name every check the scenario runs (all four when the scenario leaves `checks` out), and no other check. |

Expectations:

| Value | Met when |
|---|---|
| `pass_every_run` | The check passed in every planned run. |
| `fail_every_run` | The check failed in every planned run. |
| `fail_at_least_once` | The check failed in at least one run. |

Use `fail_at_least_once` for races that do not lose every time, such as two servers completing at the same moment.

## What `oversold expect` does

1. Validate the file. Pick the Medusa version: `--version X`, or the newest folder under `targets/medusa/versions/` when it is left out. Every case runs on that version, whatever `target.version` its scenario names.
2. Create `runs/<UTC time>-expect-<version>/`.
3. Make sure the Medusa image and the template database for that version exist, building them if needed, and time each step.
4. Run each case as `oversold run` would, with the same resets, histories, checks and reports, writing its run folder inside the expect folder.
5. Write `summary.json` and `summary.md` in the expect folder, print the table and one line for each expectation that was not met, and print the summary paths last.

`oversold versions` prints the Medusa versions Oversold can run, oldest first, one per line. It runs the same folder checks as `oversold run` and exits 2 if a version folder is invalid.

## Summary

`summary.md` and `summary.json` hold one row per case and check: the fail count out of the planned runs, the expectation, and whether it was met. They also hold the Medusa version, the image ID, the Oversold commit and whether its tree had uncommitted changes, how long the image and the template database took to build (or that they were already present), and the total time. Errors are masked like histories before they are written.

An expectation that was not met gets one line, for example:

```
single-cart-over-demand on 2.23.0: no_oversell failed in 0 of 3 runs, expected every run. Medusa may have changed this; run the full scenario set before saying so.
```

A few runs on one machine cannot show that a bug is fixed. Run every scenario with its full run count before saying so.

## Exit codes

| Code | Meaning |
|---|---|
| 0 | Every expectation was met. |
| 1 | At least one expectation was not met. |
| 2 | Invalid file or bad usage. |
| 3 | At least one run was inconclusive, a run set stopped early, or the harness failed. This wins over 1, since the counts cannot be trusted then. |

## Example

```yaml
schema: oversold.expect/v1
description: One known bug next to its control.
runs: 3
cases:
  - scenario: scenarios/medusa/single-cart-over-demand.yaml
    note: "#16502"
    expect:
      no_oversell: fail_every_run
      reserved_matches_reservations: pass_every_run
      one_order_per_cart: pass_every_run
      no_orphaned_orders: pass_every_run
  - scenario: scenarios/medusa/normal-checkout.yaml
    note: "control for #16502"
    expect:
      no_oversell: pass_every_run
      reserved_matches_reservations: pass_every_run
      one_order_per_cart: pass_every_run
      no_orphaned_orders: pass_every_run
```
