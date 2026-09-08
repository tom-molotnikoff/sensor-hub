import { useCallback } from "react";
import { useAuth } from '../providers/AuthContext.tsx';
import { apiClient } from "../gen/client";
import { useScheduledQuery } from './useScheduledQuery';
import type { TotalReadingsSample } from "../gen/aliases";

const EMPTY_SAMPLE: TotalReadingsSample = { sampled_at: '', counts: {} };

export const TOTAL_READINGS_POLL_MS = 60000;

function useTotalReadingsForEachSensor(): [TotalReadingsSample, boolean] {
  const { user } = useAuth();

  const fetcher = useCallback(async (signal: AbortSignal) => {
    const { data } = await apiClient.GET('/sensors/stats/total-readings', { signal });
    return data ?? EMPTY_SAMPLE;
  }, []);

  const { data, isLoading } = useScheduledQuery(fetcher, {
    pollIntervalMs: TOTAL_READINGS_POLL_MS,
    enabled: !!user,
    deps: [],
  });

  return [data ?? EMPTY_SAMPLE, isLoading];
}

export default useTotalReadingsForEachSensor;
