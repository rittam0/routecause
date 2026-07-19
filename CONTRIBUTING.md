# Contributing to RouteCause

Thank you for helping improve RouteCause. Keep changes focused, local-first and evidence-driven. Do not add telemetry, uploads, accounts, private identifiers or claims that exceed the measured evidence.

## Development setup

RouteCause requires Go 1.26 or newer and Node/npm for frontend development.

```sh
cd web
npm ci
npm run build
cd ..
go test ./...
```

## Before submitting a change

Run the relevant complete checks:

```sh
cd web
npm run format
npm run lint
npm test
npm run build
npm run build:portfolio
cd ..
gofmt -w cmd internal
go test ./...
go test -race ./...
go vet ./...
make build-linux
make build-windows
```

Playwright tests additionally require its pinned Chromium browser and operating-system browser libraries:

```sh
cd web
npx playwright install chromium
npm run test:e2e
```

Automated tests must use deterministic fixtures rather than the public internet. Windows collectors should be exercised through small fake native-API adapters in ordinary tests; note any validation that still requires physical Windows hardware.

## Privacy and compatibility

- Never commit raw diagnostic sessions from `experiments/results/` or `session.json`.
- Never commit SSIDs, gateway addresses, MAC addresses, BSSIDs, interface GUIDs, credentials or personal filesystem paths.
- Preserve versioned schema fields and compatibility identifiers unless a migration is explicitly designed.
- Keep WSL and unsupported platforms honest: unsupported evidence must remain typed and must never be fabricated.
- Use “simulated clients,” not “users,” for load-harness results.

## Changes and reviews

Explain the problem, the evidence behind the change, tests performed and remaining limitations. Keep generated frontend assets synchronized with their source by running `npm run build`. Do not include unrelated formatting or generated files.

By contributing, you agree that your contribution is licensed under the repository's MIT License.
