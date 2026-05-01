# Contributing to ubgo/goerr

Thanks for your interest in `ubgo/goerr`. This repository is licensed under the **Apache License 2.0**. Pull requests are welcome.

## Workflow

1. Open an issue first for anything beyond a tiny fix.
2. Fork + branch named after the issue: `fix/123-...`, `feat/456-...`.
3. Run local checks: `task ci`.
4. Use Conventional Commits for the PR title.

## Code conventions

- **Race detector mandatory.** Every test must pass under `-race`.
- **Coverage target.** ≥ 90% line coverage.
- **Public API stability.** Once the module reaches v1.0.0, breaking changes require a major version bump.

## Testing locally

```sh
task test           # standard tests
task test:race      # with race detector
task test:coverage  # with coverage report
task lint           # golangci-lint
task ci             # everything
```

## License of contributions

By submitting a pull request, you agree that your contribution is provided under the same Apache License 2.0 as the rest of the repository.
