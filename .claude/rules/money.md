---
paths:
  - "app/payments/**"
  - "app/orders/**"
  - "app/checkout/**"
  - "app/cart/**"
---

# Money and payments rules

- Money is `int64` cents (`*_cents` columns, `*Cents` JSON). Currency BRL. Never floats.
- The server prices everything: cart and checkout ignore any client price, fee or shipping value.
- Every money event (charge, PIX, transfer, refund, payout) writes one append-only `ledger_entries` row in the same transaction as the state change.
- Webhooks: verify signature first, then store the provider event id (unique) and process idempotently; mark processed only after success so retries work.
- Outbound provider calls carry deterministic idempotency keys (`checkout-<orderID>`, `transfer-<sellerOrderID>`, `refund-<sellerOrderID>`).
- State transitions are conditional updates (`UPDATE ... WHERE status = 'pending'`) and are covered by abuse tests (replay, double fulfilment, price tampering).
