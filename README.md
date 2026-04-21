# docs-cli-content

Generated content artifacts for the MongoDB Docs CLI.

This repository is an artifact repository, not a source repository.

Do not edit files here manually. Artifact files are generated and overwritten by the ingestion pipeline in [mongodb-docs-cli](https://github.com/grove-platform/mongodb-docs-cli).

## Source of Truth

- Source code and pipeline logic live in the mongodb-docs-cli repository.
- This repository only stores published build outputs consumed by the CLI.

## Expected Artifact Layout

- v1/nav.json — Navigation tree
- v1/products.json — Product metadata
- v1/{product}/{path}/summary.json — Page summary
- v1/{product}/{path}/sections/{slug}.json — Section content
- v1/{product}/{path}/full.json — Full page content
- v1/{product}/{path}/examples.json — Code examples

## Publishing

Artifacts are published from the pipeline using output to this repo path and optional automated push flow.

## CCA (CLI Content Audit)

`cca` is a local Go utility that reports on and validates the generated content. Source lives in [tools/cca/](tools/cca/).

### Build

```
cd tools/cca
go build -o cca .
```

### Commands

- `cca report` — Prints statistics about the content: page counts, sections per page (min/max/median/mean/stddev/P90), token estimates, code example counts by language, deepest heading level, and pages with zero sections. Broken down per-product and overall.
- `cca validate` — Checks that every `summary.json`, `full.json`, `sections/*.json`, and `examples.json` file is valid JSON and has the required fields the docs CLI expects. Nav-only directories (those without content files) are skipped and counted separately. Exits non-zero if any errors are found.

### Flags

- `--root <path>` — Path to the repo root. Auto-detected when run from within the repo.
- `--product <id>` — Filter to a specific product (e.g. `atlas`, `node`).

### Examples

```
./tools/cca/cca report
./tools/cca/cca report --product node
./tools/cca/cca validate
```
