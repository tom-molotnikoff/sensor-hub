import {useCallback, useEffect, useState} from "react";
import {useAuth} from '../providers/AuthContext.tsx';
import { apiClient } from "../gen/client";
import { logger } from '../tools/logger';
import type { TotalReadingsSample } from "../gen/aliases";

const emptySample: TotalReadingsSample = { sampled_at: '', counts: {} };

function useTotalReadingsForEachSensor(): [TotalReadingsSample, () => Promise<void>] {
  const [sample, setSample] = useState<TotalReadingsSample>(emptySample);
  const {user} = useAuth();

  const fetchTotalReadings = useCallback(() =>
    apiClient.GET('/sensors/stats/total-readings')
      .then(({ data }) => setSample(data ?? emptySample))
      .catch((err) => logger.error("Failed to load total readings for each sensor", err)),
  []);

  useEffect(() => {
    if (user === undefined) return;
    if (user === null) return;
    void fetchTotalReadings();
  }, [fetchTotalReadings, user]);

  return [sample, fetchTotalReadings];
}

export default useTotalReadingsForEachSensor;
