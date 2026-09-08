import type { SensorHealthHistory } from "../gen/aliases";
import { useCallback } from "react";
import { apiClient } from "../gen/client";
import { useAuth } from '../providers/AuthContext.tsx';
import { useScheduledQuery } from './useScheduledQuery';

const NO_HISTORY: SensorHealthHistory[] = [];

function useSensorHealthHistory(sensorName: string): [SensorHealthHistory[], () => Promise<void>, boolean] {
  const { user } = useAuth();

  const fetcher = useCallback(async (signal: AbortSignal) => {
    const { data } = await apiClient.GET('/sensors/health/{name}', {
      params: { path: { name: sensorName } },
      signal,
    });
    return data ?? NO_HISTORY;
  }, [sensorName]);

  const { data, isLoading, refetch } = useScheduledQuery(fetcher, {
    enabled: !!sensorName && !!user,
    deps: [sensorName],
  });

  return [data ?? NO_HISTORY, refetch, isLoading];
}

export default useSensorHealthHistory;
