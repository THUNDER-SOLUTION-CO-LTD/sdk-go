# Contributing

Use a runtime listed in `.github/workflows/ci.yml`. Install the development tools, then run:

```sh
go mod download
test -z "$(gofmt -l .)"
go vet ./...
go test -race ./...
staticcheck ./...
golangci-lint run
go build ./...
```

The test suite checks the README examples and conformance fixtures without credentials.
For live tests, set `THUNDER_API_KEY_FILE` to your own key file and set
`THUNDER_V1_BASE_URL` and `THUNDER_V2_BASE_URL` to the API endpoints:

```sh
THUNDER_ENDPOINT_INTEGRATION=1 go test -run '^TestLiveEndpoints$' .
```

Live tests may use quota. Never commit keys or slip data.
Keep pull requests focused, add regression tests and update both READMEs when the API changes.
Run the checks above before submitting. Use Conventional Commits for commit messages and PR titles.
Report vulnerabilities through [SECURITY.md](SECURITY.md).
Maintainers follow [RELEASING.md](RELEASING.md).
