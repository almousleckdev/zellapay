# HTTP reliability

Applies to the `api` and `operator-gateway` binaries. The worker serves no
HTTP.

## Middleware order

`httpx.Harden` wraps each binary's mux, outermost first:

1. Request ID and access log (`httpx.Middleware`)
2. Panic recovery (`httpx.Recover`)
3. Body limits (`httpx.BodyLimits`)
4. The mux, then each module's own guards

A recovered panic is logged as `panic recovered` with `request_id`, `stack`
and `response_committed`. The client gets the standard `internal_error`
envelope, unless the response had already started, in which case nothing more
is written.

## Body limits

The default is 1 MiB for every route, including `/callbacks/{operator}`.
Declared `Content-Length` over the limit is answered 413 before any handler
runs. Any other oversized body also ends as 413 with code
`request_too_large`.

To allow more on one route, add its registered pattern to
`BodyLimits.Routes` where the server is composed, for example
`"POST /v1/payouts/batch": 4 << 20`. Handlers may still apply a stricter
`http.MaxBytesReader` of their own.

## Shutdown order

On SIGTERM or SIGINT:

1. `/readyz` starts returning 503. `/healthz` keeps returning 200.
2. The process waits `ZP_DRAIN_DELAY` (default `0s`) so load balancers stop
   sending traffic. Set it above the balancer's health-check interval.
3. The listener closes and in-flight requests get `ZP_SHUTDOWN_TIMEOUT`
   (default `20s`) to finish.
4. The in-process simulator stops and is waited for.
5. Database and Redis connections close.

If the shutdown timeout expires, `Serve` returns the error and the process
exits non-zero.

## Readiness

`/readyz` checks PostgreSQL and Redis together under a 2 second deadline
(`httpx.ReadyTimeout`). The Redis client ignores context deadlines, so a hung
Redis check is abandoned at the deadline and finishes in the background when
the client's own read timeout fires.

## Server timeouts

Defined once in `config.Server`: read header 5s, read 15s, write 30s, idle 60s.
