# Operator adapter rules

Operators are slow, unreliable and poorly documented. Assume the worst.
Apply the root `CLAUDE.md` plus the rules below.

## Contract

- Implement `Adapter` exactly. Adapters are the only code that knows an
  operator's wire format, codes and quirks.
- Normalise every operator status into `State`. Keep the raw code and body
  for audit with secrets and personal data redacted.
- A timeout, reset, empty or malformed reply returns `ErrUnknownOutcome`.
  Never map it to a failure.
- No retries inside an adapter. The caller owns retry policy.
- Every call has a deadline from `ctx` and a response size limit.

## Security

- Credentials are read from the secret manager at start and held in memory
  only. Never log them. Never put them in errors.
- Verify every inbound callback: signature if provided, source IP allowlist,
  amount and reference match, and replay protection.
- A callback is a hint, not a fact. Confirm important state with `Status`
  before posting money.
- Outbound requests use TLS verification and pinned base URLs. Never follow
  redirects to another host.

## Tests required

- A fixture-driven test per operator reply shape, including malformed ones.
- Simulator scenarios: success, failure, slow, duplicate callback, missing
  callback, wrong amount, timeout.
- Contract test shared by all adapters, run against the simulator.
