# Variant format v1

A build variant is a Medusa image built from one pinned Medusa version plus the source change of one pull request, so the change can be run through the same scenarios as the release it patches. A variant lives in `targets/medusa/variants/<name>/`:

```
targets/medusa/variants/<name>/
  variant.yaml     what the variant is built from, decoded strictly
  files/           compiled files that replace the published ones in the image
```

`files/` holds each replacement at its path under `node_modules` in the image, for example `files/@medusajs/inventory/dist/services/inventory-module.js`. The image build copies them over the published files and needs no access to GitHub. Nothing else belongs in the folder.

```sh
./bin/oversold variant                 list the variants
./bin/oversold variant pr-16961        check one and print its fields
./bin/oversold run scenarios/medusa/two-carts-shared-stock.yaml --version 2.22.0 --variant pr-16961
scripts/build-variant.sh pr-16961      rebuild its files from source and compare them with variant.yaml
```

## Decoding rules

- The file holds exactly one YAML document.
- Decoding is strict. A field that is not listed below is an error, at any level.
- `oversold variant <name>` prints every problem it finds and exits 2 if there is at least one.

## Fields

| Field | Type | Required | Rules |
|---|---|---|---|
| `schema` | string | yes | Must be `oversold.variant/v1`. |
| `name` | string | yes | The folder name. Kebab-case, at most 40 characters. |
| `base_version` | string | yes | A Medusa version with a folder under `targets/medusa/versions/`. The variant only runs on that version. |
| `description` | string | yes | One line of at most 200 characters that names the pull request as `#<number>` and its author's login. |
| `source.pr` | string | yes | The pull request URL, `https://github.com/<owner>/<repo>/pull/<number>`. |
| `source.author` | string | yes | The pull request author's GitHub login. |
| `source.state` | string | yes | `open` or `closed`, as GitHub reported it when the variant was made. |
| `source.merged` | boolean | yes | Whether the pull request was merged. A merged pull request is `closed`. |
| `source.head_sha` | string | yes | The full commit SHA of the pull request head the files are built from. |
| `source.base_sha` | string | yes | The full commit SHA the pull request's change is measured from: the merge base of its branch. The change is `git diff base_sha head_sha` over `source_files`. |
| `source_files` | list | yes | The Medusa source files the change touches, each a `.ts` file under `packages/*/src/`. Test files (`__tests__`, `__fixtures__`, `__mocks__`, `integration-tests`, `*.spec.*`, `*.test.*`) are not allowed, since only non-test source is applied. |
| `files[].path` | string | yes | A compiled `.js` file under `@medusajs/<package>/dist/`, relative to `/server/node_modules` in the image. Each path once. |
| `files[].replaces_sha256` | string | yes | SHA-256 of the published file in the `base_version` image. Oversold checks it in that image before it builds the variant image, and refuses to build if it differs. |
| `files[].sha256` | string | yes | SHA-256 of the replacement in `files/`. Oversold checks it when it reads the variant, and again inside every running Medusa container of every run. |

## How the files are made

`scripts/build-variant.sh <name>` makes the files from source and never by hand:

1. Fetch the pull request's repository at tag `v<base_version>`, `source.base_sha` and `source.head_sha` into `.cache/` (gitignored), with a shallow fetch.
2. Take `git diff base_sha head_sha` over `source_files` and apply it to the `v<base_version>` tree. If it does not apply cleanly, the script stops.
3. Compile every package that holds a source file twice, once unpatched and once patched, the way Medusa's package build does: `tsc --build`, then the `resolve:aliases` step (`tsc --showConfig` and `tsc-alias`) for packages whose build script runs it, with Medusa's `_tsconfig.base.json`. The compile runs inside the `base_version` image, which supplies Node and the `@medusajs` packages the sources import. TypeScript, `tsc-alias`, `@types/node`, `@types/jest` and `expect-type` are taken at exactly the versions Medusa's `yarn.lock` resolves at that tag ([scripts/variant-compile.js](../scripts/variant-compile.js)).
4. Method check: every unpatched `.js` file must be byte-identical to the published file in the image, and every published `.js` file of those packages must have been compiled. Compiled test files that the packages do not publish are counted and skipped. If any file differs, the script stops.
5. The compiled `.js` files that differ between the patched and unpatched compile must be exactly the files listed in `files`.
6. Without `--write`, each patched file must match `files[].sha256` and the committed file in `files/`, and each published file must match `files[].replaces_sha256`. With `--write`, the script copies the patched files into `files/` and prints the `files` entries to put in `variant.yaml`.

The script needs Docker, Git, Go and network access to GitHub and npm, and the `base_version` image, which `oversold run` builds the first time that version runs. It exits 0 only when every check passes.

## Image

The variant image is `<prefix>:<base_version>-<name>`, for example `oversold-medusa:2.22.0-pr-16961`. It is built from the `variant` stage of `targets/medusa/Dockerfile`: the `base_version` image with `files/` copied into `/server/node_modules/`. It has its own template database, `medusa_template_<version>_<name>` with dots and dashes as underscores. Only `.js` files are replaced. The published `.d.ts` and `.js.map` files stay as they were.
