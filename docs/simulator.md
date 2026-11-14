# Operator simulator

The simulator is a scripted mobile money operator for development, tests and
the sandbox. It implements the same adapter contract as a real operator and
passes the shared contract suite. Behaviour is chosen by the **last two digits**
of the customer's phone number.

Use any number the router sends to the simulator, such as `+24107000000`
followed by the two digits below (for example `+24107000003`).

| Last digits | Behaviour |
|---|---|
| `00` and any other | Succeeds after the configured delay |
| `01` | The customer declines (`CUSTOMER_DECLINED`) |
| `02` | The customer has insufficient funds (`INSUFFICIENT_FUNDS`) |
| `03` | Initiation **times out**, yet the operator processed it and it later succeeds |
| `04` | Stays pending forever |
| `05` | Succeeds; the callback is delivered twice |
| `06` | Succeeds; no callback is ever delivered |
| `07` | Succeeds; the callback reports a different amount |
| `08` | The operator rejects the request at initiation |
| `09` | The customer never answers; the operator reports it expired (collections) |

The same numbers drive payouts: `00` pays, `01` and `08` fail and return the held
funds, `03` is resolved by the status query, `04` stays held, `05` is a duplicate
callback, `07` is held for a person.

## What each scenario proves

| Scenario | Behaviour of ZellaPay it exercises |
|---|---|
| `03` | An unclear outcome is never a failure; the status query resolves it |
| `04` | A collection is never expired by ZellaPay itself; it is flagged after 24 hours |
| `05` | A duplicate callback never records the money twice |
| `06` | The sweeper finds the result without any callback |
| `07` | A mismatched amount holds the collection for a person and posts nothing |
| `08` | A definite rejection fails the collection immediately |
| `09` | Only the operator can expire a collection |

## Callbacks

Callbacks are signed with HMAC-SHA256 over the raw body, sent in the
`X-Sim-Signature` header. The simulator exposes `DueCallbacks()`, which a
development harness delivers to the callback endpoint. Verification is done by
`ParseCallback`, which rejects tampered, unsigned or malformed callbacks.

The simulator is never enabled in production.
