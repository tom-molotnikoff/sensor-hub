import type { SensorHealthHistory } from "../gen/aliases";
import {useCallback, useEffect, useState} from "react";
import { apiClient } from "../gen/client";
import { useAuth } from '../providers/AuthContext.tsx';
import { logger } from '../tools/logger';

function useSensorHealthHistory(sensorName: string): [SensorHealthHistory[], () => Promise<void>, boolean] {
  const [healthHistory, setHealthHistory] = useState<SensorHealthHistory[]>([]);
  const [isLoading, setIsLoading] = useState(true);

  const fetchHistory = useCallback(async () => {
    setIsLoading(true);
    try {
      const { data } = await apiClient.GET('/sensors/health/{name}', {
        params: { path: { name: sensorName } },
      });
      setHealthHistory(data ?? []);
    } catch (err) {
      logger.error("Failed to load sensor health history", err);
    } finally {
      setIsLoading(false);
    }
  }, [sensorName]);

  const { user } = useAuth();

  useEffect(() => {
    if (user === undefined) return;
    if (user === null) return;
    if (!sensorName) {
      setIsLoading(false);
      return;
    }
    void fetchHistory();
  }, [fetchHistory, user, sensorName]);

  return [healthHistory, fetchHistory, isLoading];
}

export default useSensorHealthHistory;
