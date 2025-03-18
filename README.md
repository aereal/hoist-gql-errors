[![status][ci-status-badge]][ci-status]
[![PkgGoDev][pkg-go-dev-badge]][pkg-go-dev]

> [!WARNING]
> This module is no longer maintained, its functionality has been merged into [aereal/otelgqlgen](https://github.com/aereal/otelgqlgen), please use that instead.

# hoist-gql-errors

hoist-gql-errors extracts [GraphQL][] errors from the downstream response and record them as [OpenTelemetry][] exception events.

It aims at that Datadog [Error Tracking][] collects errors from OpenTelemetry traces.

Datadog Error Tracking needs error types, error messages, and error stacktraces on service entry spans (top-level spans).

In typical web applications, top-level spans mean spans that started on `http.Handler`.
In addition, [otelhttp.Handler][] will record no exceptions from downstream spans.

These facts mean that Datadog Error Tracking tracks no errors from the spans started by default setup with otelhttp.Handler.

hoist-gql-errors **hoists** (downstream) GraphQL errors and records them as service entry spans' errors, and then Datadog Error Tracking correctly collects them.

## Synopsis

See examples on [pkg.go.dev][pkg-go-dev].

```go
import (
  "net/http"

  "github.com/aereal/hoist-gql-errors"
)

func main() {
  var handler http.Handler
  var _ http.Handler = hoistgqlerrors.New()(handler)
}
```

## Installation

```sh
go get github.com/aereal/hoist-gql-errors
```

## License

See LICENSE file.

[pkg-go-dev]: https://pkg.go.dev/github.com/aereal/hoist-gql-errors
[pkg-go-dev-badge]: https://pkg.go.dev/badge/aereal/hoist-gql-errors
[ci-status-badge]: https://github.com/aereal/hoist-gql-errors/workflows/CI/badge.svg?branch=main
[ci-status]: https://github.com/aereal/hoist-gql-errors/actions/workflows/CI
[graphql]: https://graphql.org/
[opentelemetry]: https://opentelemetry.io/
[error tracking]: https://docs.datadoghq.com/tracing/error_tracking/
[otelhttp.Handler]: https://pkg.go.dev/go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp#Handler
