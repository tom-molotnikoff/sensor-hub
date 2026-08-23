import { useEffect, useState, useCallback } from 'react';
import type { ApiKey } from '../gen/aliases';
import { apiClient } from '../gen/client';
import { useAuth } from '../providers/AuthContext';
import { logger } from '../tools/logger';

export function useApiKeys() {
  const [apiKeys, setApiKeys] = useState<ApiKey[]>([]);
  const [loaded, setLoaded] = useState(false);
  const { user } = useAuth();

  const load = useCallback(() =>
    apiClient.GET('/api-keys')
      .then(({ data }) => setApiKeys(data ?? []))
      .catch((err) => {
        logger.error('Failed to load API keys', err);
        setApiKeys([]);
      })
      .finally(() => setLoaded(true)),
  []);

  const refresh = useCallback(() => {
    setLoaded(false);
    return load();
  }, [load]);

  useEffect(() => {
    if (user === undefined || user === null) return;
    void load();
  }, [user, load]);

  return { apiKeys, loaded, refresh };
}
