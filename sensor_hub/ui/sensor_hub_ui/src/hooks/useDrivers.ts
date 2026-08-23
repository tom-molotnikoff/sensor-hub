import { useEffect, useState, useCallback } from 'react';
import type { DriverInfo } from '../gen/aliases';
import { apiClient } from '../gen/client';
import { logger } from '../tools/logger';

export function useDrivers(type?: 'pull' | 'push') {
  const [drivers, setDrivers] = useState<DriverInfo[]>([]);
  const [loaded, setLoaded] = useState(false);

  const load = useCallback(() =>
    apiClient.GET('/drivers', { params: { query: type ? { type } : undefined } })
      .then(({ data }) => setDrivers(data ?? []))
      .catch((err) => logger.error('Failed to fetch drivers:', err))
      .finally(() => setLoaded(true)),
  [type]);

  const refresh = useCallback(() => {
    setLoaded(false);
    return load();
  }, [load]);

  useEffect(() => {
    void load();
  }, [load]);

  return { drivers, loaded, refresh };
}
