import { useEffect, useRef, useState, useMemo } from "react";
import type { ChartEntry, Sensor, Reading } from "../gen/aliases";
import type { DateTime } from "luxon";
import { apiClient } from "../gen/client";
import { requestScheduler, type RequestPriority } from "../scheduler/requestScheduler";
import { logger } from '../tools/logger';

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
  measurementType?: string;
  aggregationFunction?: string;
  /** When provided, called on every poll tick to get fresh dates (for relative presets like "last 24h"). */
  resolveTimeRange?: () => ResolvedRange;
  onDataUpdate?: (date: Date) => void;
}

export function useReadingsData({
                                     startDate,
                                     endDate,
                                     sensors,
                                     pollIntervalMs = 30000,
                                     measurementType,
                                     aggregationFunction,
                                     resolveTimeRange,
                                     onDataUpdate,
                                   }: useReadingsDataProps) {
  const [mergedData, setMergedData] = useState<ChartEntry[]>([]);
  const [aggregation, setAggregation] = useState<AggregationMeta>({ interval: 'raw', function: 'none' });
  const prevResponseJsonRef = useRef<string | null>(null);
  const prevSensorsKeyRef = useRef<string>("");
  const prevMergedJsonRef = useRef<string>("");
  const requestIdRef = useRef(0);
  const isMountedRef = useRef(true);

  // Stable string key for sensor list — use this as effect dep instead of the array
  const sensorsKey = useMemo(() => sensors.map((s) => s.name).join("|"), [sensors]);

  // Keep mutable values in refs to avoid effect dependency churn
  const sensorsRef = useRef(sensors);
  sensorsRef.current = sensors;

  const resolveTimeRangeRef = useRef(resolveTimeRange);
  resolveTimeRangeRef.current = resolveTimeRange;

  const onDataUpdateRef = useRef(onDataUpdate);
  onDataUpdateRef.current = onDataUpdate;

  // When a resolver is provided, don't use rendered dates as effect deps
  // (DateTime.now() changes every render, causing infinite re-runs).
  // Use a stable key: 'resolver' for callback-based, or the ISO strings for static dates.
  const startIso = startDate?.toUTC().toISO() ?? null;
  const endIso = endDate?.toUTC().toISO() ?? null;
  const timeKey = resolveTimeRange ? 'resolver' : `${startIso}|${endIso}`;

  useEffect(() => {
    isMountedRef.current = true;

    // Resolve dates for the initial fetch
    const resolved = resolveTimeRangeRef.current?.();
    const initStart = resolved?.startDate?.toUTC().toISO() ?? startIso;
    const initEnd = resolved?.endDate?.toUTC().toISO() ?? endIso;
    if (!initStart || !initEnd) return;

    const fetchAndMaybeUpdate = async (
      fetchStartIso: string,
      fetchEndIso: string,
      force = false,
      priority: RequestPriority = 'normal',
    ) => {
      const currentRequestId = ++requestIdRef.current;
      const currentSensors = sensorsRef.current;
      const currentSensorsKey = currentSensors.map((s) => s.name).join("|");
      try {
        const response = await requestScheduler.schedule(priority, () => apiClient.GET('/readings/between', {
          params: { query: { start: fetchStartIso, end: fetchEndIso, type: measurementType, aggregation_function: aggregationFunction as never } },
        }));
        const data: Reading[] = response.data?.readings ?? [];

        if (requestIdRef.current !== currentRequestId || !isMountedRef.current) return;

        setAggregation({
          interval: response.data?.aggregation_interval ?? 'raw',
          function: response.data?.aggregation_function ?? 'none',
        });

        const dataJson = JSON.stringify(data);

        if (!force && prevResponseJsonRef.current === dataJson && prevSensorsKeyRef.current === currentSensorsKey) {
          return;
        }

        const times = Array.from(
          new Set((data).map((r) => r.time.replace(" ", "T") + "Z"))
        );

        const newMergedData: ChartEntry[] = times.map((time) => {
          const entry: ChartEntry = { time };
          currentSensors.forEach((sensor) => {
            const found = data.find(
              (r) => r.sensor_name === sensor.name && r.time.replace(" ", "T") + "Z" === time
            );
            entry[sensor.name] = found
              ? found.numeric_value != null
                ? found.numeric_value
                : found.text_state === 'true' ? 1 : found.text_state === 'false' ? 0 : null
              : null;
          });
          return entry;
        });

        const newMergedJson = JSON.stringify(newMergedData);

        if (newMergedJson !== prevMergedJsonRef.current) {
          prevMergedJsonRef.current = newMergedJson;
          setMergedData(newMergedData);
          onDataUpdateRef.current?.(new Date());
        }

        prevResponseJsonRef.current = dataJson;
        prevSensorsKeyRef.current = currentSensorsKey;
      } catch (err: unknown) {
        if (!isMountedRef.current) return;
        logger.error("Error fetching readings:", err);
      }
    };

    void fetchAndMaybeUpdate(initStart, initEnd, true, 'normal');

    const intervalId = window.setInterval(() => {
      // Re-resolve time range on each tick so relative presets slide forward
      const tick = resolveTimeRangeRef.current?.();
      const freshStart = tick?.startDate?.toUTC().toISO() ?? initStart;
      const freshEnd = tick?.endDate?.toUTC().toISO() ?? initEnd;
      // Background polls are low priority so they yield to initial loads and commands.
      void fetchAndMaybeUpdate(freshStart, freshEnd, false, 'low');
    }, pollIntervalMs);

    return () => {
      requestIdRef.current = Number.MAX_SAFE_INTEGER;
      isMountedRef.current = false;
      window.clearInterval(intervalId);
    };
    // timeKey is stable for resolver-based callers ('resolver'), or changes when static dates change.
    // sensorsKey changes when the sensor list changes.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [measurementType, aggregationFunction, sensorsKey, pollIntervalMs, timeKey]);

  return { mergedData, aggregation };
}
