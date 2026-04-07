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
