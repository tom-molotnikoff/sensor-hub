import { useCallback, useContext, useEffect, useState } from 'react';
import type { MeasurementTypeInfo } from '../gen/aliases';
import { apiClient } from '../gen/client';
import { logger } from '../tools/logger';
import {
  MeasurementTypesContext,
  type MeasurementTypeListName,
} from '../providers/MeasurementTypesContext';

function useMeasurementTypeList(list: MeasurementTypeListName, enabled: boolean): MeasurementTypeInfo[] {
  const { lists, request } = useContext(MeasurementTypesContext);

  useEffect(() => {
    if (enabled) request(list);
  }, [enabled, list, request]);

  return lists[list];
}

export function useMeasurementTypes(enabled = true): MeasurementTypeInfo[] {
  return useMeasurementTypeList('all', enabled);
}

export function useMeasurementTypesWithReadings(enabled = true): MeasurementTypeInfo[] {
  return useMeasurementTypeList('withReadings', enabled);
}

const NO_MEASUREMENT_TYPES: MeasurementTypeInfo[] = [];

export function useSensorMeasurementTypes(sensorId: number | null) {
  const [fetchedTypes, setFetchedTypes] = useState<MeasurementTypeInfo[]>([]);
  const [fetchDone, setFetchDone] = useState(false);

  // Reset the loaded flag when the sensor changes, so a stale list never reads as loaded.
  const [prevSensorId, setPrevSensorId] = useState(sensorId);
  if (prevSensorId !== sensorId) {
    setPrevSensorId(sensorId);
    setFetchDone(false);
  }

  const load = useCallback(() => {
    if (sensorId === null) return Promise.resolve();
    return apiClient.GET('/sensors/by-id/{id}/measurement-types', {
      params: { path: { id: sensorId } },
    })
      .then(({ data }) => setFetchedTypes(data ?? []))
      .catch((err) => logger.error('Failed to fetch sensor measurement types:', err))
      .finally(() => setFetchDone(true));
  }, [sensorId]);

  const refresh = useCallback(() => {
    if (sensorId !== null) setFetchDone(false);
    return load();
  }, [load, sensorId]);

  useEffect(() => {
    void load();
  }, [load]);

  // With no sensor there is nothing to fetch: an empty, already-loaded list.
  const measurementTypes = sensorId === null ? NO_MEASUREMENT_TYPES : fetchedTypes;
  const loaded = sensorId === null ? true : fetchDone;

  return { measurementTypes, loaded, refresh };
}
