import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import type { ChartEntry, Sensor } from "../gen/aliases";
import type { DateTime } from "luxon";
import { apiClient } from "../gen/client";
import { mergeReadings, readingsFingerprint } from "../readings/mergeReadings";
import { useScheduledQuery } from "./useScheduledQuery";

interface ResolvedRange {
  startDate: DateTime<boolean> | null;
  endDate: DateTime<boolean> | null;
}

export interface AggregationMeta {
  interval: string;
  function: string;
}

interface useReadingsDataProps {
  startDate: DateTime<boolean> | null;
  endDate: DateTime<boolean> | null;
  sensors: Sensor[];
  pollIntervalMs?: number;
  enabled?: boolean;
  measurementType?: string;
  aggregationFunction?: string;
  /** When provided, called on every poll tick to get fresh dates (for relative presets like "last 24h"). */
  resolveTimeRange?: () => ResolvedRange;
  onDataUpdate?: (date: Date) => void;
}

const RAW_AGGREGATION: AggregationMeta = { interval: 'raw', function: 'none' };
const NO_ROWS: ChartEntry[] = [];

export function useReadingsData({
                                     startDate,
                                     endDate,
                                     sensors,
                                     pollIntervalMs = 30000,
                                     enabled = true,
                                     measurementType,
                                     aggregationFunction,
                                     resolveTimeRange,
                                     onDataUpdate,
                                   }: useReadingsDataProps) {
  const sensorsKey = useMemo(() => sensors.map((s) => s.name).join("|"), [sensors]);

  const sensorsRef = useRef(sensors);
  const resolveTimeRangeRef = useRef(resolveTimeRange);
  const onDataUpdateRef = useRef(onDataUpdate);
  useEffect(() => {
    sensorsRef.current = sensors;
    resolveTimeRangeRef.current = resolveTimeRange;
    onDataUpdateRef.current = onDataUpdate;
  });

  const startIso = startDate?.toUTC().toISO() ?? null;
  const endIso = endDate?.toUTC().toISO() ?? null;
  const timeKey = resolveTimeRange ? 'resolver' : `${startIso}|${endIso}`;

  const fetcher = useCallback(async (signal: AbortSignal) => {
    const resolved = resolveTimeRangeRef.current?.();
    const start = resolved?.startDate?.toUTC().toISO() ?? startIso;
    const end = resolved?.endDate?.toUTC().toISO() ?? endIso;
    if (!start || !end) return null;

    const { data } = await apiClient.GET('/readings/between', {
      params: { query: { start, end, type: measurementType, aggregation_function: aggregationFunction as never } },
      signal,
    });
    return data ?? null;
  }, [startIso, endIso, measurementType, aggregationFunction]);

  const { data, error, isLoading } = useScheduledQuery(fetcher, {
    pollIntervalMs,
    enabled,
    deps: [measurementType, aggregationFunction, sensorsKey, timeKey],
  });

  const [mergedData, setMergedData] = useState<ChartEntry[]>(NO_ROWS);
  const fingerprintRef = useRef<string | null>(null);

  useEffect(() => {
    if (data === undefined) return;
    const rows = mergeReadings(data?.readings ?? [], sensorsRef.current);
    const fingerprint = readingsFingerprint(rows, sensorsRef.current);
    if (fingerprint === fingerprintRef.current) return;
    fingerprintRef.current = fingerprint;
    setMergedData(rows);
    onDataUpdateRef.current?.(new Date());
  }, [data, sensorsKey]);

  const aggregation: AggregationMeta = data
    ? { interval: data.aggregation_interval ?? 'raw', function: data.aggregation_function ?? 'none' }
    : RAW_AGGREGATION;

  return {
    mergedData,
    aggregation,
    isLoading,
    error: error instanceof Error ? error.message : error != null ? String(error) : null,
  };
}
