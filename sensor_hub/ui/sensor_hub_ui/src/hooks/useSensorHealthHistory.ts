import type { SensorHealthHistory } from "../gen/aliases";
import {useCallback, useEffect, useState} from "react";
import { apiClient } from "../gen/client";
import { useAuth } from '../providers/AuthContext.tsx';
import { logger } from '../tools/logger';

function useSensorHealthHistory(sensorName: string): [SensorHealthHistory[], () => Promise<void>, boolean] {
  const [healthHistory, setHealthHistory] = useState<SensorHealthHistory[]>([]);
  const [fetching, setFetching] = useState(true);

  // Reset the fetching flag when the sensor changes, so stale data never reads as loaded.
  const [prevName, setPrevName] = useState(sensorName);
  if (prevName !== sensorName) {
    setPrevName(sensorName);
    setFetching(true);
  }

  const load = useCallback(() => {
    if (!sensorName) return Promise.resolve();
    return apiClient.GET('/sensors/health/{name}', {
      params: { path: { name: sensorName } },
    })
      .then(({ data }) => setHealthHistory(data ?? []))
      .catch((err) => logger.error("Failed to load sensor health history", err))
      .finally(() => setFetching(false));
  }, [sensorName]);

  const fetchHistory = useCallback(() => {
    setFetching(true);
    return load();
  }, [load]);

  // With no sensor there is nothing to load.
  const isLoading = !!sensorName && fetching;

  const { user } = useAuth();

  useEffect(() => {
    if (user === undefined) return;
    if (user === null) return;
    void load();
  }, [user, load]);

  return [healthHistory, fetchHistory, isLoading];
}

export default useSensorHealthHistory;
