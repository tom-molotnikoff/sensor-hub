import { useCallback } from "react";
import { useAuth } from '../providers/AuthContext.tsx';
import { apiClient } from "../gen/client";
import { useScheduledQuery } from './useScheduledQuery';
import type { TotalReadingsSample } from "../gen/aliases";

const EMPTY_SAMPLE: TotalReadingsSample = { sampled_at: '', counts: {} };

function useTotalReadingsForEachSensor(pollIntervalMs?: number): [TotalReadingsSample, boolean] {
  const { user } = useAuth();

  const fetcher = useCallback(async (signal: AbortSignal) => {
    const { data } = await apiClient.GET('/sensors/stats/total-readings', { signal });
    return data ?? EMPTY_SAMPLE;
  }, []);

  const { data, isLoading } = useScheduledQuery(fetcher, {
    pollIntervalMs,
    enabled: !!user,
    deps: [],
  });

  return [data ?? EMPTY_SAMPLE, isLoading];
}

export default useTotalReadingsForEachSensor;
