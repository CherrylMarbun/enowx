# Contributing to enowX

Thanks for your interest! enowX is in early development, so expect things to move fast.

## Before you start

- Read [docs/PLAN.md](docs/PLAN.md) for scope and principles. Features outside the plan's scope (see "Non-goals") will likely be declined.
- For anything larger than a small fix, open an issue or discussion first.

## Development

Requirements: Go 1.27+ and git.

```sh
make build   # build binaries into bin/
make test    # run tests
make lint    # go vet + staticcheck
make fmt     # gofmt
```

Code conventions are in [AGENTS.md](AGENTS.md). They apply to humans and AI agents alike.

## Pull requests

- Keep PRs focused on one change.
- Make sure `make test` and `make lint` pass.
- Sign off your commits (`git commit -s`) to certify the [Developer Certificate of Origin](https://developercertificate.org/).

By contributing, you agree that your contributions are licensed under the [MIT License](LICENSE).
