# Mimir

[![Tests](https://github.com/ksysoev/mimir/actions/workflows/tests.yml/badge.svg)](https://github.com/ksysoev/mimir/actions/workflows/tests.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/ksysoev/mimir)](https://goreportcard.com/report/github.com/ksysoev/mimir)
[![Go Reference](https://pkg.go.dev/badge/github.com/ksysoev/mimir.svg)](https://pkg.go.dev/github.com/ksysoev/mimir)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](https://opensource.org/licenses/MIT)

In-memory key-value store with versioning and optimistic locking

## Installation

## Building from Source

```sh
RUN CGO_ENABLED=0 go build -o mimir -ldflags "-X main.version=dev -X main.name=mimir" ./cmd/mimir/main.go
```

### Using Go

If you have Go installed, you can install Mimir directly:

```sh
go install github.com/ksysoev/mimir/cmd/mimir@latest
```


## Using

```sh
mimir --log-level=debug --log-text=true --config=runtime/config.yml
```

## License

Mimir is licensed under the MIT License. See the LICENSE file for more details.
