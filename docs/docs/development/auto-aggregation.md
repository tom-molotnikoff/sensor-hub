---
id: auto-aggregation
title: Auto-Aggregation
sidebar_position: 12
---

# Auto-Aggregation

Smart auto-aggregation automatically selects an appropriate time bucket size when querying sensor readings, keeping
response sizes manageable for charting regardless of the time range requested.

## Problem

Without aggregation, a 30-day query at a 5-minute collection interval produces ~8,640 data points per sensor. With 
multiple sensors this quickly exceeds what chart libraries like Recharts can render performantly, and the density 
of points exceeds what a user can meaningfully interpret on screen.

## Solution

Query-time aggregation using SQL `GROUP BY` with `strftime()` bucket expressions. The server inspects the time span
of each request and selects an aggregation interval from a configurable tier list. 

## Configuration

Tiers are configured as a single comma-separated property in `application.properties`.

Each entry is `THRESHOLD:INTERVAL`, meaning: "If the time span is ≤ THRESHOLD, use INTERVAL as the bucket size."
The tiers are sorted by threshold automatically. When the span exceeds all thresholds, the fallback interval `P1D`
is used.

### Default aggregation functions

Each measurement type can have a default aggregation function.

The `supported_aggregation_functions` list for each measurement type is exposed via the `GET /measurement-types` 
endpoint.

When no aggregation is applied (short range or explicit `raw`), the response uses `"aggregation_interval": "raw"`
and `"aggregation_function": "none"`.

### Aggregation functions

Each function produces one value per sensor, measurement type and bucket:

| Function | Value per bucket |
|---|---|
| `avg` | Mean of the readings, rounded to 2 decimal places |
| `min` | Lowest reading, rounded to 2 decimal places |
| `max` | Highest reading, rounded to 2 decimal places |
| `count` | Number of readings |
| `last` | The most recent reading, unchanged |

Numeric types support `avg`, `min` and `max`, defaulting to `avg`. Binary types support `count` and `last`,
defaulting to `count`.