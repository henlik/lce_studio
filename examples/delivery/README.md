# Reservation delivery references

The pharmacy and library scenarios share one exact `lce.reservation-delivery@1.0.0` model. It embeds the unchanged v0.6 `lce.resource-reservation@1.0.0` source machine and a generic `lce.reservation-ledger@1.0.0` recipient. Both use the source's exact `lce.reservation-data@1.0.0` contract and its required `ReservationNotice` record. The route maps the source effect's `notice` payload directly to the recipient command's `notice` input.

```sh
go run ./cmd/deliverycheck examples/delivery/reservation-delivery.model.json
go run ./cmd/deliverycheck --lock examples/delivery/reservation-delivery.lock.json
go run ./cmd/deliverycheck --run examples/delivery/pharmacy-reservation.run.json
go run ./cmd/deliverycheck --run examples/delivery/library-reservation.run.json
```

The source owns durable availability. The recipient independently owns a durable reserved total. Both have volatile acceptance counters. The source counter starts at zero; the recipient overrides its initial baseline to ten, so restarts visibly reset it to ten. Tenant/resource identifiers match within each example; this explicit matching does not authenticate a caller or implement production tenancy.

Pharmacy availability starts at five units and its first reservation takes two. Library availability starts at four copies and its first reservation takes one. Both therefore have three available after the first command. Later reservations take one and two units, exhausting stock. The intended recipient totals are five and four respectively. Quantities and identities are synthetic.

## Handwritten ordered oracle

Steps are zero-based, matching the engine evidence. Every dispatch and delivery is explicit. A committed source intention does not automatically execute the recipient.

| Step | Action | Independent expected outcome |
| --- | --- | --- |
| 0 | Source `reserveA`, revision 0, first quantity | Commit; availability 3, source counter 1, revision 1, one intention |
| 1 | Dispatch `firstReservation` from `reserveA`, target revision 0 | Freeze one pending envelope; attempts 0; no recipient work |
| 2 | Deliver first message; `drop_before_receive` | Dropped; attempts 1; recipient total 0, revision 0, no receipt |
| 3 | Deliver first message; `before_recipient_commit` | Recipient rolls back; attempts 2; total 0 and counter 10 unchanged |
| 4 | Deliver first message; `after_recipient_commit_before_ack` | Recipient commits first quantity, counter 11, revision 1 and receipt; acknowledgement lost; attempts 3, message pending |
| 5 | Restart recipient | Total, revision and receipt survive; counter resets to 10 |
| 6 | Restart source | Availability 3, revision and intention survive; counter resets to 0 |
| 7 | Deliver first message without fault | Replay original recipient receipt; attempts 4; acknowledge it; current counter stays 10 while the historical receipt says 11 |
| 8 | Retry original source `reserveA` | Replay its original receipt; no new source change or intention |
| 9 | Redispatch first message with original target revision 0 | `dispatch_replayed`; preserve the acknowledged envelope and attempts 4 |
| 10 | Source `reserveB`, revision 1, quantity 1 | Commit; availability 2, counter 1, revision 2 |
| 11 | Dispatch `secondReservation` from `reserveB`, target revision 1 | One new pending envelope; attempts 0 |
| 12 | Deliver second message | Commit and acknowledge; recipient total 3 for pharmacy or 2 for library; counter 11, revision 2 |
| 13 | Source `reserveC`, revision 2, quantity 2 | Commit; availability 0, counter 2, revision 3 |
| 14 | Dispatch `thirdReservation` from `reserveC`, target revision 2 | One new pending envelope; attempts 0 |
| 15 | Deliver third message | Commit and acknowledge; recipient total 5 or 4; counter 12, revision 3 |
| 16 | Source `reserveD`, revision 3, quantity 1 | Denied; store receipt without changing state/revision or creating an intention |
| 17 | Retry original source `reserveD` | Replay denied receipt; no new work or intention |
| 18 | Restart recipient | Final total survives; counter resets to 10; all transport acknowledgements survive |

| Final owner | Durable quantity | Volatile counter | Revision | Receipts | Effect intentions |
| --- | ---: | ---: | ---: | ---: | ---: |
| Source, both examples | Available 0 | 2 | 3 | 4 | 3 |
| Pharmacy recipient | Total reserved 5 | 10 | 3 | 3 | 0 |
| Library recipient | Total reserved 4 | 10 | 3 | 3 | 0 |

| Final message | Status | Attempts | Recipient business verdict |
| --- | --- | ---: | --- |
| `firstReservation` | acknowledged | 4 | accepted |
| `secondReservation` | acknowledged | 1 | accepted |
| `thirdReservation` | acknowledged | 1 | accepted |

There are six delivery attempts but only three committed recipient commands. Source replays and redispatch create no additional envelopes or business effects. Four source evaluations use 21 operations each, including the denied command. Four recipient evaluations use 22 operations each, including the rolled-back proposal. Expected completed work is therefore **172 primitive operations and eight invocations**.

## Pure recipient boundaries

The recipient reducer checks matching identifiers, positive quantity, nonnegative initial total and remaining safe-integer capacity. It selects a safe total before calculating headroom and a safe delta before addition, because pure selection is eager. Eight embedded cases have explicit expected outputs: normal acceptance, wrong tenant, wrong resource, zero quantity, extreme negative quantity, exhausted numeric capacity, exact upper boundary and extreme negative initial total. Counter overflow remains an ordinary explicit evaluation error.

Deduplication comes from the recipient's durable command receipt, not from the pure reducer or a collection scan. Message identity retains a full SHA-256 digest encoded as a valid generated command ID. Retry preserves the original recipient revision and typed input map. A stale frozen revision cannot be silently refreshed by redispatch.

These experiments assume durable transport metadata across owner restarts. They do not run a broker, deliver external messages, create a distributed transaction, model elapsed time or reserve hardware/network resources. Each run starts fresh; PostgreSQL retains immutable experiment evidence. An acknowledged receipt may also represent a business denial in other scenarios: transport acknowledgement and acceptance are distinct assertions.
