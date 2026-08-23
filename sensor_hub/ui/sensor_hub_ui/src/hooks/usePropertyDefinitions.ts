import { useEffect, useState } from 'react';
import { apiClient } from '../gen/client';
import type { PropertyDefinitionsResponse } from '../gen/aliases';

// Definitions cannot change without a deploy, so one fetch serves the whole session.
let cached: PropertyDefinitionsResponse | null = null;

export function usePropertyDefinitions() {
  const [definitions, setDefinitions] = useState<PropertyDefinitionsResponse | null>(cached);
  const [loading, setLoading] = useState(cached === null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (cached !== null) return;
    let cancelled = false;
    (async () => {
      try {
        const { data, error: apiError } = await apiClient.GET('/properties/definitions');
        if (cancelled) return;
        if (data) {
          cached = data;
          setDefinitions(data);
        } else {
          setError(typeof apiError === 'string' ? apiError : 'Failed to load property definitions');
        }
      } catch {
        if (!cancelled) setError('Failed to load property definitions');
      } finally {
        if (!cancelled) setLoading(false);
      }
    })();
    return () => {
      cancelled = true;
    };
  }, []);

  return { definitions, loading, error };
}
