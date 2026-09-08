# sensor-hub dashboard first-load latency: trace investigation

**Source:** OpenObserve `default` org, `default` traces/logs streams, `db_sql_*` metric streams.
**Window analysed:** 2026-08-25 → 2026-09-08 (14 days). All times UTC.
**Service:** `sensor-hub` v1.3.3~dev2, Go 1.26.4, linux/arm64 (Raspberry Pi, Debian 12).

---

## 0. Verdict up front

The ~9 second dashboard first load is **one deterministically slow SQL query serialising every other request through a single database connection**.

| | |
|---|---|
| **Root cause** | `GET /api/measurement-types?has_readings=true` → `MeasurementTypeRepositoryImpl.GetAllWithReadings` runs `SELECT DISTINCT … FROM measurement_types mt INNER JOIN readings r ON r.measurement_type_id = mt.id`. There is **no index on `readings(measurement_type_id)`**, so SQLite full-scans the multi-million-row `readings` table to answer a question with ~8 rows of output. It takes **8.78–9.29 s, every single time (8/8 executions in 14 days)**. |
| **Amplifier** | `db.SetMaxOpenConns(1)` (`db/db.go:45`), confirmed by the metric `db_sql_connection_max_open = 1`. The 9 s query holds the *only* connection, so every other request the dashboard fires at the same instant blocks in `database/sql`'s connection queue for the full 9 s and inherits the same latency. |
| **Ruled out** | SQLite lock contention (`SQLITE_BUSY` / "database is locked" appear **0 times** in 14 days of logs — max_open=1 makes them impossible by construction). Slow `readings/between` SQL (it is 144 ms). VACUUM/retention (they stall the app badly, but at *different* wall-clock times than the dashboard episodes). |
| **User's "10-15 queries before the real one"** | Confirmed and located — it is the auth middleware chain (5 queries × 4 concurrent requests ≈ 20 queries), but it is **not the problem**: those queries total **~2.5 ms**. The user correctly saw a queue; the queue is caused by the measurement-types query, not the auth chain. |

---

## 1. Slow episodes over 14 days

310 spans exceeded 3 s, forming 170 clusters. They fall into **three distinct, unrelated populations**:

### 1a. Periodic VACUUM (161 clusters — the majority, but NOT the dashboard problem)

Every 2 hours, at `HH:02:03` (`HH:01:56` before the 2026-08-31 restart), a single `sql.conn.exec` runs **`VACUUM`**:

- **161 occurrences in 14 days**, duration **7.12 s – 11.72 s**.
- 100% of the 161 `sql.conn.exec` spans over 3 s are `VACUUM`. Nothing else.
- Driven by `service/cleanup_service.go:184` (`data.cleanup.interval.hours`, default 1, evidently configured to 2).
- Log evidence, `cleanup_service` component, 161 × `database stats before vacuum` / `after vacuum`.

Latest stats logged (2026-09-08 08:02:04): `size_bytes=101908480` (102 MB), `page_count=24880`, `page_size=4096`, **`freelist_count=218`, `freelist_bytes=892928`**.

> **Surprising:** VACUUM rewrites the entire 102 MB database and stalls the whole application for ~9 s in order to reclaim **892 KB** (0.9% of the file). This runs 12 times a day. It blocks `mqtt.handle_message` in about half the occurrences.

These clusters occur at fixed 2-hourly instants and **do not coincide with any dashboard-load episode**, so they are a separate defect, not the cause of the reported symptom.

### 1b. Dashboard page loads (7 clusters — this IS the reported problem)

| # | Wall clock (UTC) | Root operations in flight | Concurrent slow reqs | Max duration |
|---|---|---|---|---|
| 1 | 2026-08-26 16:50:23 | measurement-types ×2, readings/between ×2, properties/ws ×2 | 4 + 4 (two loads 24 s apart) | 9.08 s |
| 2 | 2026-08-28 11:19:38 | measurement-types, readings/between ×4, properties/ws | 6 | 9.62 s |
| 3 | 2026-08-28 11:54:32 | measurement-types, readings/between, properties/ws | 3 | 9.41 s |
| 4 | 2026-08-31 15:11:57 | measurement-types, readings/between ×4, properties/ws | 6 | 9.89 s |
| 5 | 2026-09-02 15:22:17 | measurement-types, readings/between ×4, notifications/ws | 6 | 9.86 s |
| 6 | 2026-09-08 09:20:20 | measurement-types, readings/between ×2, properties/ws | 4 | 9.48 s |
| 7 | 2026-09-08 09:23:54 | measurement-types, readings/between ×2, properties/ws | 4 | 9.51 s |

**Every one of these clusters contains exactly one `GET /api/measurement-types`.** There is no dashboard-load episode without it, and there is no `measurement-types?has_readings=true` call that is not slow.

### 1c. Daily process restart at 05:01 (2 clusters)

The process restarts daily around 05:01 (`process_pid` changes; `start_time` on the otelsql metrics = `1788757274205437445` = 2026-09-07 05:01:14). On the cold page cache immediately after:

| Time | Span | Duration | Statement |
|---|---|---|---|
| 2026-09-07 05:01:15.704 | `sql.conn.query` (trace `b633d6b533e989ffed833985ef2e2a51`, span `54ba4ac2964e855c`) | **43.53 s** | latest-reading-per-sensor window query (below) |
| 2026-09-07 05:01:59.237 | `sql.rows` (trace `27cd701062e1001b`) | 4.42 s | (same family) |
| 2026-09-07 05:02:04.729 | `sql.conn.exec` (trace `d3276cb6d1fb26e8`) | 13.94 s | `VACUUM` |
| 2026-08-31 05:01:16 | `sql.conn.query` (trace `dfc375a8049f02fa85a164f52a887177`, span `8fc2b98e038178ff`) | **41.03 s** | same |

```sql
SELECT sub.id, sub.sensor_name, sub.measurement_type, sub.numeric_value, sub.text_state, sub.unit, sub.time
FROM (
    SELECT r.id, s.name AS sensor_name, mt.name AS measurement_type,
        r.numeric_value, r.text_state, COALESCE(smt.unit, mt.default_unit) AS unit, r.time,
        ROW_NUMBER() OVER (PARTITION BY r.sensor_id, r.measurement_type_id ORDER BY r.time DESC) AS rn
    FROM readings r
    JOIN sensors s ON r.sensor_id = s.id
    JOIN measurement_types mt ON r.measurement_type_id = mt.id
    LEFT JOIN sensor_measurement_types smt ON smt.sensor_id = s.id AND smt.measurement_type_id = mt.id
) sub
WHERE sub.rn = 1
```

This ranks **every row in `readings`** to return one row per (sensor, measurement type) — the same anti-pattern as the measurement-types query. No `WHERE`, no `LIMIT`. Warm it is fast enough to stay under the 3 s probe threshold; cold it is 43 s. If the user's first dashboard load of the day happens shortly after 05:01, this is a second, worse stall.

---

## 2. Full trace reconstructions

### Episode 2026-09-08 09:20:20 — four requests, one connection

All four start at **09:20:20.322Z** (same millisecond) and all release at **09:20:29.49Z** (same instant).

#### Trace `a46b2ada3db337a289204784e4b37fe2` — `GET /api/measurement-types`, 9.172 s — **THE HOLDER**

| Offset | Duration | Span | span_id | SQL |
|---|---|---|---|---|
| +0.00 ms | 9171.95 ms | `GET /api/measurement-types` | `83f5e3237e69dbfe` | *(root)* |
| +0.06 | 0.00 | `sql.conn.reset_session` | `0f9cf74a20220c1b` | |
| +0.09 | 0.17 | `sql.conn.query` | `7ea211abbe0d30cb` | `SELECT user_id, expires_at FROM sessions WHERE token_hash = ?` |
| +0.26 | 0.04 | `sql.rows` | `4d4cf00dec022e87` | |
| +0.63 | 0.00 | `sql.conn.reset_session` | `a459bbcad0ee3679` | |
| +0.64 | 0.11 | `sql.conn.exec` | `dc43d276e5bfebe5` | `UPDATE sessions SET last_accessed_at = ? WHERE token_hash = ?` |
| +0.85 | 0.00 | `sql.conn.reset_session` | `c923bc9cb358a09b` | |
| +0.86 | 0.06 | `sql.conn.query` | `e0ed085d242839eb` | `SELECT id, username, email, must_change_password, disabled, created_at, updated_at FROM users WHERE id = ?` |
| +0.92 | 0.02 | `sql.rows` | `73832604f443c464` | |
| +1.10 | 0.00 | `sql.conn.reset_session` | `09507a153f8e1551` | |
| +1.11 | 0.07 | `sql.conn.query` | `4534383a8468a319` | `SELECT r.name FROM roles r JOIN user_roles ur ON r.id = ur.role_id WHERE ur.user_id = ?` |
| +1.17 | 0.01 | `sql.rows` | `f0a6a5207cbcb586` | |
| +1.86 | 0.00 | `sql.conn.reset_session` | `38a7c8ec1532b615` | |
| +1.87 | 0.07 | `sql.conn.query` | `8fc016ce9b17d1f4` | `SELECT p.name FROM permissions p JOIN role_permissions rp … WHERE ur.user_id = ?` |
| +1.93 | 0.05 | `sql.rows` | `1fe26ab89e805155` | |
| +2.14 | 0.00 | `sql.conn.reset_session` | `fad347f48440f7cc` | |
| +2.15 | **0.31** | `sql.conn.query` | `ca6cf30b3bcbf557` | **the measurement-types query (prepare/step-0 only)** |
| +2.46 | **9168.70** | `sql.rows` | `4bbd5a59a81ff0b6` | **row iteration — 9.17 s** |

**Waterfall:**
```
GET /api/measurement-types  |==================================================| 9171.95ms
 auth chain (8 spans)       |#                                                 |    2.14ms
 sql.conn.query (mt query)  | |                                                |    0.31ms
 sql.rows                   |  ================================================| 9168.70ms  <-- 99.97%
```
**No gap.** 99.97% of this request is inside a single `sql.rows` span. The work is genuinely happening in SQLite, not waiting.

> Why `sql.rows` and not `sql.conn.query`: SQLite computes rows lazily on `sqlite3_step()`. `QueryContext` returns almost instantly (0.31 ms); the full-table scan is paid during `rows.Next()`, which otelsql attributes to `sql.rows`. This is the definitive signature of a slow query plan, not of lock waiting.

The query (`db/measurement_type_repository.go:49-63`, `GetAllWithReadings`):

```sql
SELECT DISTINCT mt.id, mt.name, mt.display_name, mt.category, mt.default_unit,
    COALESCE(mta.function, 'avg') AS default_aggregation_function,
    COALESCE((SELECT GROUP_CONCAT(mta2.function, ',') FROM measurement_type_aggregations mta2
              WHERE mta2.measurement_type_id = mt.id ORDER BY mta2.function), 'avg')
        AS supported_aggregation_functions
FROM measurement_types mt
INNER JOIN readings r ON r.measurement_type_id = mt.id
LEFT JOIN measurement_type_aggregations mta ON mta.measurement_type_id = mt.id AND mta.is_default = 1
ORDER BY mt.name
```

No `WHERE`, no `LIMIT`. `INNER JOIN readings` + `DISTINCT` is an existence test ("which types have ≥1 reading") implemented as a full cross-join-and-dedupe over the entire readings history.

#### Trace `8a02f495175e0173964910d32ec1cb64` — `GET /api/readings/between`, 9.326 s — **VICTIM**

| Offset | Duration | Span | SQL |
|---|---|---|---|
| +0.00 ms | 9325.82 ms | `GET /api/readings/between` (`44767fa60f9fd382`) | *(root)* |
| +0.49 → +1.81 | 8 spans, ~0.3 ms total | auth chain | sessions / users / roles / permissions (identical to above) |
| **+1.81 → +9171.23** | — | **GAP: 9169.4 ms with no child span** | **blocked in `database/sql` waiting for the single connection** |
| +9171.23 | 0.01 | `sql.conn.reset_session` | |
| +9171.26 | 0.31 | `sql.conn.query` | `SELECT mta.function, mta.is_default FROM measurement_type_aggregations mta JOIN measurement_types mt ON mta.measurement_type_id = mt.id WHERE LOWER(mt.name) = LOWER(?)` |
| +9171.57 | 0.04 | `sql.rows` | |
| +9172.00 | 0.10 | `sql.conn.query` | `SELECT id FROM measurement_types WHERE LOWER(name) = LOWER(?)` |
| +9172.10 | 0.02 | `sql.rows` | |
| +9172.72 | **144.03** | `sql.conn.query` | **the actual readings query (below)** |
| +9316.75 | 5.68 | `sql.rows` | |

```sql
SELECT 0 AS id, s.name, mt.name, ROUND(AVG(r.numeric_value), 2), NULL,
       COALESCE(smt.unit, mt.default_unit),
       strftime('%Y-%m-%d %H:', r.time) || printf('%02d', (CAST(strftime('%M', r.time) AS INTEGER) / 15) * 15) || ':00' AS bucket_time
FROM readings r
JOIN sensors s ON r.sensor_id = s.id
JOIN measurement_types mt ON r.measurement_type_id = mt.id
LEFT JOIN sensor_measurement_types smt ON smt.sensor_id = s.id AND smt.measurement_type_id = mt.id
WHERE r.time BETWEEN ? AND ? AND r.measurement_type_id = ?
GROUP BY s.name, mt.name, bucket_time ORDER BY bucket_time ASC
```

**Waterfall:**
```
GET /api/readings/between   |==================================================| 9325.82ms
 auth chain                 |#                                                 |    1.81ms
 (waiting for connection)   | ..............................................   | 9169.42ms  <-- 98.3%
 aggregation lookups        |                                              |   |    1.50ms
 readings query             |                                               ==|  149.71ms
```
**The user's own query is 150 ms. The other 9169 ms is queueing.** This directly confirms "normally <100 ms, 8 s on first load".

#### Trace `ff5dbde2a1fde030bc243acb183cb170` — `GET /api/readings/between`, 9.479 s — **VICTIM, and shows the serialisation twice**

| Offset | Duration | Span | Note |
|---|---|---|---|
| +0.00 | 9478.60 ms | `GET /api/readings/between` (`efab7f858c4ec9ae`) | root |
| +0.31 → +2.09 | 8 spans | auth chain | ~0.35 ms of SQL |
| **+2.09 → +9172.32** | — | **GAP 9170.2 ms** | waiting for the connection held by measurement-types |
| +9172.33 | 0.17 | `sql.conn.query` | aggregation function lookup |
| **+9172.52 → +9322.56** | — | **GAP 150.0 ms** | **waiting again — this time behind trace `8a02f495`'s 149.71 ms readings query** |
| +9322.58 | 0.18 | `sql.conn.query` | `SELECT id FROM measurement_types WHERE LOWER(name) = LOWER(?)` |
| +9322.96 | **146.64** | `sql.conn.query` | its own readings query |
| +9469.59 | 5.65 | `sql.rows` | |

The second 150 ms gap is an exact match for the sibling request's query duration. This is unambiguous proof of strict serialisation on one connection.

#### Trace `1692ec7ff85c46035b0fbe242976a8ee` — `GET /api/properties/ws`, 9.476 s — **VICTIM, blocked mid-auth**

| Offset | Duration | Span | SQL |
|---|---|---|---|
| +0.00 | 9476.38 ms | `GET /api/properties/ws` (`96001c72bc9afe8d`) | root |
| +1.23 | 0.09 | `sql.conn.query` | `SELECT user_id, expires_at FROM sessions WHERE token_hash = ?` |
| +1.53 | 0.08 | `sql.conn.exec` | `UPDATE sessions SET last_accessed_at = ? WHERE token_hash = ?` |
| **+1.61 → +9172.17** | — | **GAP 9170.6 ms** | it lost the connection race *inside* the auth middleware |
| +9172.18 | 0.14 | `sql.conn.query` | `SELECT id, username, … FROM users WHERE id = ?` |
| +9172.57 | 0.07 | `sql.conn.query` | `SELECT r.name FROM roles r JOIN user_roles ur …` |
| **+9172.65 → +9475.79** | — | **GAP 303.1 ms** | queued behind the two readings/between queries (149.7 + 146.6 = 296 ms) |
| +9475.80 | 0.18 | `sql.conn.query` | `SELECT p.name FROM permissions p JOIN role_permissions rp …` |

#### Collateral: `mqtt.handle_message` trace `192d3f389b134dbb9cf086d0d8ca0ea3`, 4.821 s (109 spans)

Started 09:20:24.984, i.e. mid-stall. Its **first** SQL span is at **+4509.99 ms** — releasing at 09:20:29.494Z, the *same instant* the four HTTP requests released. It then does all 109 spans of real work in 311 ms. Sensor ingestion is stalled by the dashboard load.

### Episode 2026-09-08 09:23:54 (identical shape, 3.5 minutes later)

| Time | Duration | Operation | trace_id |
|---|---|---|---|
| 09:23:54.185 | **9.196 s** | `GET /api/measurement-types` | `cd8b3b22b07fbe26b788f58d1ce101e1` |
| 09:23:54.185 | 9.507 s | `GET /api/readings/between` | `9550334ec9a889bf…` |
| 09:23:54.185 | 9.353 s | `GET /api/readings/between` | `fa0531b24c89f13d…` |
| 09:23:54.200 | 9.335 s | `GET /api/properties/ws` | `3015134fb3c33fb9…` |

The measurement-types `sql.rows` span here is 9.19 s (parent `194755f3dc36e989`). Same signature exactly.

---

## 3. What is actually blocking — the pool, proven from metrics

`db/db.go`:
```go
dsn := fmt.Sprintf("file:%s?_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)&_pragma=synchronous(NORMAL)", dbPath)
db, err := sql.Open(driverName, dsn)
...
db.SetMaxOpenConns(1)      // line 45
```

Metric confirmation (otelsql 0.42.0, `instrumentation_library_name = github.com/XSAM/otelsql`):

| Metric | Value |
|---|---|
| `db_sql_connection_max_open` | **1.0** |
| `db_sql_connection_open{status="idle"}` | 1.0 |
| `db_sql_connection_open{status="inuse"}` | 0.0 (at idle) |

**`db_sql_connection_wait_duration` (cumulative ms) around the 09:20:20 episode:**

| Minute ending | Cumulative | Δ per minute |
|---|---|---|
| 09:19:14 | 147430.474 | — |
| 09:20:14 | 147442.336 | **11.9 ms** (baseline) |
| **09:21:14** | **185589.657** | **38 147.3 ms** ← the episode |
| 09:22:14 | 185602.990 | 13.3 ms |
| 09:23:14 | 185618.275 | 15.3 ms |
| **09:24:14** | **222311.924** | **36 693.6 ms** ← second episode |
| 09:25:14 | 222705.826 | 393.9 ms |

**`db_sql_connection_wait` (cumulative count of goroutines that had to wait):**

| Minute ending | Δ waiters |
|---|---|
| 09:20:14 | 13 |
| **09:21:14** | **206** |
| 09:22:14 | 14 |
| 09:23:14 | 16 |
| **09:24:14** | 65 |

38.1 seconds of aggregate connection wait in a single minute against a baseline of ~12 ms — a **3200×** spike, exactly aligned with the episode, and arithmetically equal to the four blocked requests (9.17 + 9.33 + 9.48 + 9.48 ≈ 37.5 s, plus MQTT and websocket pollers).

**Time inside `sql` spans vs. between them:**

| Request | Inside sql spans | Between (pool wait) |
|---|---|---|
| `GET /api/measurement-types` (`a46b2ada…`) | **9171.2 ms (99.97%)** | ~0 |
| `GET /api/readings/between` (`8a02f495…`) | 151.5 ms (1.6%) | **9169.4 ms (98.3%)** |
| `GET /api/readings/between` (`ff5dbde2…`) | 152.7 ms (1.6%) | **9320.2 ms (98.3%)** |
| `GET /api/properties/ws` (`1692ec7f…`) | 0.6 ms (0.006%) | **9473.7 ms (99.99%)** |

One request does all the work; three do none and wait for it.

---

## 4. Correlation with writes and background jobs

**In the same seconds as episode 09:20:20:** nothing. The only other activity is `mqtt.handle_message` (trace `192d3f389b134dbb…`) which is itself a *victim* — its first SQL span sits at +4510 ms, blocked until the exact release instant. No retention job, no VACUUM, no checkpoint runs at 09:20.

**Ingest rate (for scale):** `INSERT INTO readings` executed **99 242 times on 2026-09-07** (~1.15 writes/sec sustained). Each MQTT message inserts ~9 readings (`readings=9` attribute on the `processed MQTT message` debug log). With 90-day default retention, `readings` is a multi-million-row table — which is what the `INNER JOIN readings` scan traverses.

**Background jobs found:**

| Job | Cadence | Cost | Blocks everything? |
|---|---|---|---|
| `VACUUM` (`service/cleanup_service.go:184`) | every 2 h | 7.1–11.7 s (13.9 s at 05:02) | **Yes** — holds the single connection |
| retention deletes (`deleted old sensor readings`, `retention_days=90`, `custom_sensors=8`) | every 2 h | not individually slow | no |
| health-history / failed-login / notification / alert-history cleanup | every 2 h | fast | no |
| `collect-all-sensors` | 3860 in 14 d (~every 5 min) | max 4.21 s | occasionally |
| daily process restart ~05:01 + cold latest-readings query | daily | 41–43.5 s | **Yes** |

**Log search, full 14 days (`?type=logs`):**

| Term | Hits |
|---|---|
| `locked` | **0** |
| `SQLITE_BUSY` | **0** |
| `busy` | **0** |
| `checkpoint` | 0 |
| `timeout` | 0 |
| `vacuum` | 322 (161 before + 161 after, INFO, `cleanup_service`) |

Severity distribution: 3 027 865 DEBUG, 6557 INFO, **78 WARN**, **0 ERROR**. The only recurring warning is `smtp_notifier: OAuth not configured, skipping notification email` (77×) — unrelated.

**This rules out SQLite lock contention entirely.** There is no `SQLITE_BUSY`, no busy-timeout retry, no "database is locked" — and there cannot be, because `SetMaxOpenConns(1)` means only one goroutine ever touches SQLite at a time. The serialisation is happening one layer up, in Go's `database/sql` connection queue.

---

## 5. What normal looks like

### `GET /api/readings/between` — 639 samples over 14 days

| Metric | Value |
|---|---|
| min | 13.3 ms |
| **p50** | **185.5 ms** |
| p90 | 318.7 ms |
| p95 | 342.8 ms |
| p99 | 9530.2 ms |
| max | 9893.9 ms |
| > 100 ms | 635 / 639 (99.4%) |
| **> 1 s** | **20 / 639 (3.13%)** |
| > 3 s | 19 / 639 (2.97%) |

The distribution is **bimodal**: a tight cluster at 13–350 ms, then a cliff straight to 9.3–9.9 s with nothing in between. There is no gradual tail. That is the signature of a binary condition (blocked / not blocked), not of variable query cost.

All 20 of the >1 s samples fall inside the seven dashboard-load episodes in §1b.

A **fast** `readings/between` (trace `cf66706a394ca875`, 2026-09-08 09:20:59.194, **15 ms**) has the same span structure as the slow one — 8 auth spans + 3 lookup queries + the aggregation query — just with no gaps. **Span count is identical between fast and slow traces (24 spans).** No extra span appears only in slow traces; the difference is entirely the two gaps.

### `GET /api/measurement-types` — 9 samples over 14 days

| Duration | Which query |
|---|---|
| 8781 ms | `has_readings=true` |
| 9018 ms | `has_readings=true` |
| 9021 ms | `has_readings=true` |
| 9172 ms | `has_readings=true` |
| 9196 ms | `has_readings=true` |
| 9228 ms | `has_readings=true` |
| 9261 ms | `has_readings=true` |
| 9288 ms | `has_readings=true` |
| **839 ms** | plain `GetAll` (no `INNER JOIN readings`) |

- `SELECT count(*) WHERE db_statement LIKE '%INNER JOIN readings%'` over 14 days = **8**. All eight map 1:1 to the eight ≥8.78 s responses.
- **p50 = p95 = p99 ≈ 9.2 s. 8 of 9 (89%) exceed 1 s. 100% of `has_readings=true` calls exceed 8.7 s.**
- The one 839 ms sample (trace `75acd3cd8a7d2de2d9cb1a1aad52abd4`, 09:20:59.194) is the *plain* variant, and even it is 99.9% queueing: its own SQL is `sql.conn.query` 0.24 ms + `sql.rows` 0.29 ms, preceded by an **830 ms gap** waiting for the connection. Its real cost is **0.53 ms**.

That 0.53 ms vs 9171 ms is the whole story: the same table, the same output, with and without `INNER JOIN readings`.

---

## 6. Dashboard load fan-out

All HTTP server spans (`span_kind='2'`) in the 4 seconds around the 2026-09-08 09:20:20 load — **21 requests**:

| Time | Duration | Endpoint |
|---|---|---|
| 09:20:19.283 | 0.023 s | `GET` (static asset) |
| 09:20:19.333 | 0.159 s | `GET` (static asset) |
| 09:20:19.333 | 0.447 s | `GET` (static asset) |
| 09:20:20.044 | 0.000 s | `GET` (static asset) |
| 09:20:20.055 | 0.069 s | `GET /api/auth/me` |
| 09:20:20.144 | 0.088 s | `GET` (static asset) ×2 |
| 09:20:20.232 | 0.008 s | `GET /api/sensors/ws` |
| 09:20:20.232 | 0.039 s | `GET /api/notifications` |
| 09:20:20.232 | 0.008 s | `GET /api/dashboards` |
| 09:20:20.232 | 0.007 s | `GET /api/notifications/unread-count` |
| 09:20:20.232 | 0.040 s | `GET /api/notifications/preferences` |
| 09:20:20.258 | 0.014 s | `GET /api/notifications/ws` |
| 09:20:20.294 | 0.000 / 0.017 s | `GET` (static asset) ×2 |
| 09:20:20.303 | 0.001 s | `GET /api/properties/ws` |
| **09:20:20.322** | **9.172 s** | **`GET /api/measurement-types`** |
| **09:20:20.322** | **9.476 s** | **`GET /api/properties/ws`** |
| **09:20:20.322** | **9.326 s** | **`GET /api/readings/between`** |
| **09:20:20.322** | **9.479 s** | **`GET /api/readings/between`** |
| 09:20:20.339 | 0.010 s | `GET` (static asset) |

**Counts:** `GET` static ×9, `/api/readings/between` ×2, `/api/properties/ws` ×2, and one each of `/api/auth/me`, `/api/sensors/ws`, `/api/notifications`, `/api/dashboards`, `/api/notifications/unread-count`, `/api/notifications/preferences`, `/api/notifications/ws`, `/api/measurement-types`.

The first wave (`.055`–`.303`) is the app shell and all completes in under 90 ms. **The second wave at `.322` is the dashboard widgets, and it is exactly this wave that contains measurement-types.** The frontend fires them in parallel; one connection turns parallel into serial.

A second, larger burst at 09:20:59.194 (after the user's dashboard rendered) issues **5 × `readings/between`, 2 × `sensors/health/:name`, `sensors/by-id/:id/measurement-types`, `measurement-types`, `properties/ws`** simultaneously, and shows the same effect at smaller scale: four readings/between complete in 13–20 ms, but the fifth (trace `2b616aecd33873c3`) takes **2.157 s** because it queued behind the others.

The dashboard's measurement-types call comes from `ui/sensor_hub_ui/src/hooks/useMeasurementTypesWithReadings()` (`ui/sensor_hub_ui/src/hooks/useMeasurementTypes.ts:34`), which unconditionally requests `has_readings: true` on mount.

---

## 7. Anything surprising

1. **VACUUM reclaims 0.9 MB and costs a 9 s global stall, 12× a day.** 161 runs in 14 days, 7.1–11.7 s each, on a 102 MB database with only 218 free pages. Roughly 20 minutes per day of total application unavailability for essentially no benefit. (Trace examples: `51f5536be6190bef1f638550f919ef09` 7.74 s, `228d07636b69e3e00bbed18bd70d2cd0` 8.52 s, `d3276cb6d1fb26e8` 13.94 s.)

2. **The 05:01 daily restart costs 43.5 s.** Trace `b633d6b533e989ffed833985ef2e2a51` span `54ba4ac2964e855c` — 43.53 s for the latest-reading-per-sensor query, on cold cache, then a 13.9 s VACUUM 45 s later. Same `ROW_NUMBER() OVER` full-scan anti-pattern as the measurement-types query.

3. **N+1 inside `mqtt.handle_message`.** Trace `192d3f389b134dbb9cf086d0d8ca0ea3` has **109 spans** for one MQTT message. Per reading it re-runs `SELECT id FROM measurement_types WHERE LOWER(name) = LOWER(?)` and `SELECT id FROM sensors WHERE LOWER(name) = LOWER(?)` before each `INSERT INTO readings` — 9 readings × 2 lookups + 9 inserts. It then runs the **same alert-rules query 8 times in a row** (spans `85471151dffdb8c5`, `b229cc1faeed9b8f`, `c83dd6244e67c5e5`, `39174f3e628a38f3`, `dfdb319034e6274c`, `fcdb2d929e54d605`, `06bfa88a0b1118b2`, `ff27d6b1b1c5dd25`, `f204eda7a91679d4` — identical `SELECT ar.id, ar.sensor_id, s.name, … FROM …` text). At ~99 242 inserts/day this is ~600 000 avoidable lookups per day, each taking one turn on the single connection. Individually sub-millisecond, but it is the reason the connection is never idle for long.

4. **`LOWER()` on both sides of every lookup predicate** (`WHERE LOWER(name) = LOWER(?)`) defeats any index on `sensors.name` / `measurement_types.name`. Small tables, so cheap today, but executed ~600 k times a day.

5. **No index on `readings(measurement_type_id)`.** The only readings indexes are `idx_readings_time` (time DESC), `idx_readings_time_asc` (time ASC) and `idx_readings_sensor_type_time (sensor_id, measurement_type_id, time DESC)`. `measurement_type_id` is not a leftmost prefix of the composite, so it is unusable for `INNER JOIN readings r ON r.measurement_type_id = mt.id`. Migration `000020_drop_redundant_readings_index` explicitly reasoned about index hygiene on this table but this access path was not considered.

6. **`sql.conn.reset_session` fires before literally every statement** — 5 849 066 spans in 14 days, more than any other operation. It is 0.00 ms each so it costs nothing, but it is 39% of all trace volume and pure noise.

7. **Websocket reconnect storm after each stall.** Immediately after each episode releases, ~15 `GET /api/properties/ws` and ~10 `GET /api/readings/ws/current` fire within ~500 ms at ~23 ms intervals (e.g. 09:24:03.589 → 09:24:04.144). These are the clients that gave up during the 9 s stall retrying. `properties/ws` p50 is 0.9 ms but p95 is 1235 ms and max 9476 ms, entirely from being caught in stalls.

8. **Only 631 `readings/between` requests in 14 days vs 4.5 M `sql.conn.query` spans.** The dashboard is used rarely; the database is saturated by ingest. So the page-cache pages that the measurement-types scan needs are never warm — which is why the query is 9.17 s cold every time rather than sometimes fast.

---

## 8. Hypotheses: supported vs. ruled out

| Hypothesis | Verdict | Evidence |
|---|---|---|
| **A slow query plan (`INNER JOIN readings` + `DISTINCT` with no usable index) is the primary cost** | **SUPPORTED — this is the root cause** | 99.97% of trace `a46b2ada…` is one `sql.rows` span (9168.70 ms). 8/8 executions 8.78–9.29 s. Query text has no `WHERE`/`LIMIT`; no index on `readings(measurement_type_id)`. Same endpoint without `has_readings` costs 0.53 ms. |
| **`SetMaxOpenConns(1)` converts one slow request into a whole-page stall** | **SUPPORTED — this is the amplifier** | `db_sql_connection_max_open = 1` (metric) and `db/db.go:45`. Three victim traces show 9169–9474 ms gaps with zero child spans, releasing at the same instant the holder's `sql.rows` ends. `db_sql_connection_wait_duration` +38 147 ms in the episode minute vs 12 ms baseline, across 206 waiters. Trace `ff5dbde2…` shows a second 150 ms gap exactly matching a sibling's 149.71 ms query. |
| SQLite lock contention / `SQLITE_BUSY` / busy-timeout | **RULED OUT** | Zero occurrences of `locked`, `SQLITE_BUSY`, `busy`, `timeout` in 3 034 500 log records over 14 days. 0 ERROR-severity logs. With `max_open=1` only one goroutine can be in SQLite at a time, so SQLite-level lock contention is structurally impossible. WAL mode is enabled. |
| The `readings/between` query itself is slow | **RULED OUT** | 144.03 ms and 146.64 ms in the two slow traces; 13–20 ms warm. p50 across 639 samples is 185.5 ms. The query is fine. |
| The "10-15 queries before the real one" are the problem | **RULED OUT as the cause; CONFIRMED as an observation** | The pre-query chain is 5 auth queries per request (sessions SELECT, sessions UPDATE, users, roles, permissions) plus 2 measurement-type lookups. Across the 4 concurrent requests that is ~20 statements — matching the user's count. But they total **~2.5 ms**. They are a symptom of what the user could see in the waterfall, not the cost. |
| VACUUM / retention / cleanup causes the dashboard stall | **RULED OUT for the dashboard symptom; SEPARATE REAL DEFECT** | VACUUM runs at fixed `HH:02:03` instants. None of the seven dashboard episodes (16:50:23, 11:19:38, 11:54:32, 15:11:57, 15:22:17, 09:20:20, 09:23:54) fall at those times. VACUUM independently stalls the app 7–12 s, 12× daily, and does block MQTT ingest. |
| MQTT ingest holds the connection during page load | **RULED OUT** | The MQTT trace in the episode (`192d3f389b134dbb…`) is itself blocked — first SQL span at +4510 ms, releasing at the same instant as the HTTP requests. It is a victim, not a holder. |
| Cold OS page cache is a contributing factor | **PARTIALLY SUPPORTED** | The 05:01 post-restart query is 43.5 s vs sub-3 s warm. The dashboard is loaded ~7 times in 14 days while ~99 k inserts/day churn the cache, so the readings table is always cold when measurement-types scans it. This explains why the query is consistently ~9 s rather than sometimes fast — but the plan, not the cache, is the defect. |

---

## 9. Where the 9.4 s goes, end to end

For `GET /api/readings/between` trace `ff5dbde2a1fde030bc243acb183cb170` (9.479 s):

| Component | Time | Share |
|---|---|---|
| Its own auth chain | 0.35 ms | 0.004% |
| **Waiting for the connection held by `measurement-types`' 9168.70 ms `sql.rows`** | **9170.2 ms** | **96.7%** |
| Waiting behind the sibling `readings/between` query | 150.0 ms | 1.6% |
| Its own lookups | 0.35 ms | 0.004% |
| **Its own actual readings query** | **152.3 ms** | **1.6%** |

Fixing the measurement-types query alone removes ~96.7% of the observed latency. Raising `MaxOpenConns` above 1 (with WAL already enabled) would additionally remove the residual serialisation, but on its own it would leave the 9 s measurement-types response — the user would still see the dashboard's type selector take 9 s, just without dragging the charts down with it.

---

## 10. Trace id reference

| trace_id | Time (UTC) | Operation | Duration |
|---|---|---|---|
| `a46b2ada3db337a289204784e4b37fe2` | 2026-09-08 09:20:20.322 | `GET /api/measurement-types` (holder) | 9.172 s |
| `8a02f495175e0173964910d32ec1cb64` | 2026-09-08 09:20:20.322 | `GET /api/readings/between` (victim) | 9.326 s |
| `ff5dbde2a1fde030bc243acb183cb170` | 2026-09-08 09:20:20.322 | `GET /api/readings/between` (victim, double gap) | 9.479 s |
| `1692ec7ff85c46035b0fbe242976a8ee` | 2026-09-08 09:20:20.322 | `GET /api/properties/ws` (victim) | 9.476 s |
| `192d3f389b134dbb9cf086d0d8ca0ea3` | 2026-09-08 09:20:24.984 | `mqtt.handle_message` (victim, 109 spans) | 4.821 s |
| `75acd3cd8a7d2de2d9cb1a1aad52abd4` | 2026-09-08 09:20:59.194 | `GET /api/measurement-types` (plain variant, 0.53 ms of SQL) | 0.839 s |
| `cf66706a394ca875…` | 2026-09-08 09:20:59.194 | `GET /api/readings/between` (fast baseline) | 0.015 s |
| `2b616aecd33873c3d5f61b6c0e34b74f` | 2026-09-08 09:20:59.225 | `GET /api/readings/between` (queued behind siblings) | 2.157 s |
| `cd8b3b22b07fbe26b788f58d1ce101e1` | 2026-09-08 09:23:54.185 | `GET /api/measurement-types` (second episode holder) | 9.196 s |
| `b633d6b533e989ffed833985ef2e2a51` | 2026-09-07 05:01:15.704 | `sql.conn.query` latest-readings, cold | 43.534 s |
| `dfc375a8049f02fa85a164f52a887177` | 2026-08-31 05:01:16 | `sql.conn.query` latest-readings, cold | 41.029 s |
| `d3276cb6d1fb26e8…` | 2026-09-07 05:02:04.729 | `VACUUM` | 13.938 s |
| `51f5536be6190bef1f638550f919ef09` | 2026-09-08 09:02:04.111 | `VACUUM` | 7.740 s |

## 11. Source references

| Path | Relevance |
|---|---|
| `/Users/tommolotnikoff/Documents/personal/git/sensor-hub/sensor_hub/db/db.go:45` | `db.SetMaxOpenConns(1)` |
| `/Users/tommolotnikoff/Documents/personal/git/sensor-hub/sensor_hub/db/db.go:38` | DSN — WAL, `synchronous(NORMAL)`, no `busy_timeout` pragma |
| `/Users/tommolotnikoff/Documents/personal/git/sensor-hub/sensor_hub/db/measurement_type_repository.go:49` | `GetAllWithReadings` — the 9 s query |
| `/Users/tommolotnikoff/Documents/personal/git/sensor-hub/sensor_hub/api/sensor_api.go:451` | dispatch on `params.HasReadings` |
| `/Users/tommolotnikoff/Documents/personal/git/sensor-hub/sensor_hub/ui/sensor_hub_ui/src/hooks/useMeasurementTypes.ts:34` | frontend sends `has_readings: true` on mount |
| `/Users/tommolotnikoff/Documents/personal/git/sensor-hub/sensor_hub/service/cleanup_service.go:184` | VACUUM invocation |
| `/Users/tommolotnikoff/Documents/personal/git/sensor-hub/sensor_hub/db/maintenance_repository.go:18` | `ExecContext(ctx, "VACUUM")` |
| `/Users/tommolotnikoff/Documents/personal/git/sensor-hub/sensor_hub/db/migrations/000006_generic_sensor_model.up.sql:78-80` | readings indexes — none on `measurement_type_id` alone |
| `/Users/tommolotnikoff/Documents/personal/git/sensor-hub/sensor_hub/db/migrations/000020_drop_redundant_readings_index.up.sql` | index hygiene migration |
| `/Users/tommolotnikoff/Documents/personal/git/sensor-hub/sensor_hub/application_properties/application_configuration.go:19` | `data.cleanup.interval.hours` |
