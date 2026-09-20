# pinned, so every run resolves the same tool
govulncheck_version := "v1.8.0"

# list available recipes
default:
    @just --list

# compile every package
build:
    go build ./...

# run the tests
test:
    go test ./...

# lint and check formatting, warning first on golangci-lint version drift
lint:
    bash hack/lint.sh

# apply the formatters
fmt:
    golangci-lint fmt ./...

# check for known vulnerabilities in reachable code, needs network
vuln:
    go run golang.org/x/vuln/cmd/govulncheck@{{govulncheck_version}} ./...

# everything that must pass before a push
ci: lint build test vuln
