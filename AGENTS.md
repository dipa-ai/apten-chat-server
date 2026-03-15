# Repository Guidelines

## Project Structure & Module Organization
`cmd/server` contains the API entry point and router setup. Core application code lives under `internal/` and is organized by domain (`auth`, `chat`, `message`, `user`, `files`, `push`, `ws`, `invite`). Database access is split between handwritten query definitions in `internal/db/queries` and generated Go code in `internal/db/dbq`; treat `internal/db/dbq` as generated output. Schema changes live in `migrations/`, and deployment manifests live in `k8s/`. Local infrastructure is defined in `docker-compose.yml`.

## Build, Test, and Development Commands
Use the `Makefile` targets for the standard workflow:

- `make run` starts the server from `./cmd/server`.
- `make test` runs `go test ./...` across all packages.
- `make docker-up` starts Postgres and MinIO for local development.
- `make docker-down` stops local containers.
- `make migrate-up` / `make migrate-down` apply or roll back Goose migrations using `DATABASE_URL`.
- `make migrate-create NAME=add_feature` creates a new SQL migration.
- `make sqlc-generate` regenerates `internal/db/dbq` from `sqlc.yaml`.

Copy `.env.example` to `.env` and set `DATABASE_URL`, `JWT_SECRET`, and S3 or MinIO credentials before running locally.

## Coding Style & Naming Conventions
Follow standard Go formatting: run `gofmt -w` on changed files before opening a PR. Keep package names lowercase and short, exported identifiers in `PascalCase`, and JSON fields in `snake_case` only when the API already uses that form. Match the existing pattern of pairing `service.go` and `handler.go` inside each domain package. Do not hand-edit generated files in `internal/db/dbq`; update SQL in `internal/db/queries` and rerun `make sqlc-generate`.

## Testing Guidelines
There are currently no committed `*_test.go` files, so new features should add focused tests alongside the package they cover. Use Go’s standard `testing` package, prefer table-driven tests for handlers and services, and keep `make test` passing before submission. When a change depends on Postgres or MinIO behavior, run it against the local Compose stack.

## Commit & Pull Request Guidelines
This repository does not yet have established git history, so use short, imperative commit subjects such as `add invite expiry validation`. Keep commits scoped to one change. PRs should explain the behavior change, note any new environment variables or migrations, and include example requests or responses for API changes.

## Security & Configuration Tips
Never commit real secrets. Use `.env.example` as the baseline, keep JWT and AWS credentials local, and call out any auth, file upload, or migration risk in the PR description.
