import {useCallback, useEffect, useState} from "react";
import {useAuth} from '../providers/AuthContext.tsx';
import { apiClient } from "../gen/client";
import { logger } from '../tools/logger';


function useTotalReadingsForEachSensor(): [Record<string, number>, () => Promise<void>] {
  const [totalReadingsPerSensor, setTotalReadingsPerSensor] = useState<Record<string, number>>({});
  const {user} = useAuth();

  const fetchTotalReadings = useCallback(() =>
    apiClient.GET('/sensors/stats/total-readings')
      .then(({ data }) => setTotalReadingsPerSensor((data as Record<string, number>) ?? {}))
      .catch((err) => logger.error("Failed to load total readings for each sensor", err)),
  []);

  useEffect(() => {
    if (user === undefined) return;
    if (user === null) return;
    void fetchTotalReadings();
  }, [fetchTotalReadings, user]);

  return [totalReadingsPerSensor, fetchTotalReadings];
}

export default useTotalReadingsForEachSensor;