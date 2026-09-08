import { type ReactNode, useCallback, useMemo, useRef, useState } from 'react';
import type { MeasurementTypeInfo } from '../gen/aliases';
import { apiClient } from '../gen/client';
import { requestScheduler } from '../scheduler/requestScheduler';
import { logger } from '../tools/logger';
import {
  MeasurementTypesContext,
  type MeasurementTypeListName,
  type MeasurementTypeLists,
} from './MeasurementTypesContext';

interface MeasurementTypesProviderProps {
  children: ReactNode;
}

const LIST_QUERY: Record<MeasurementTypeListName, { has_readings?: boolean }> = {
  all: {},
  withReadings: { has_readings: true },
};

const EMPTY_LISTS: MeasurementTypeLists = { all: [], withReadings: [] };

export default function MeasurementTypesProvider({ children }: MeasurementTypesProviderProps) {
  const [lists, setLists] = useState<MeasurementTypeLists>(EMPTY_LISTS);
  const requestedRef = useRef(new Set<MeasurementTypeListName>());

  const request = useCallback((list: MeasurementTypeListName) => {
    if (requestedRef.current.has(list)) return;
    requestedRef.current.add(list);

    void requestScheduler
      .schedule('normal', () => apiClient.GET('/measurement-types', {
        params: { query: LIST_QUERY[list] },
      }))
      .then(({ data, error }) => {
        if (error) throw error;
        setLists((prev) => ({ ...prev, [list]: (data as MeasurementTypeInfo[] | undefined) ?? [] }));
      })
      .catch((err) => {
        requestedRef.current.delete(list);
        logger.error('Failed to fetch measurement types:', err);
      });
  }, []);

  const value = useMemo(() => ({ lists, request }), [lists, request]);

  return (
    <MeasurementTypesContext.Provider value={value}>
      {children}
    </MeasurementTypesContext.Provider>
  );
}
