import { useEffect, useState, useCallback } from 'react';
import type { MeasurementTypeInfo } from '../gen/aliases';
import { apiClient } from '../gen/client';
import { logger } from '../tools/logger';

export function useMeasurementTypes() {
  const [measurementTypes, setMeasurementTypes] = useState<MeasurementTypeInfo[]>([]);
  const [loaded, setLoaded] = useState(false);

  const load = useCallback(() =>
    apiClient.GET('/measurement-types')
      .then(({ data }) => setMeasurementTypes(data ?? []))
      .catch((err) => logger.error('Failed to fetch measurement types:', err))
      .finally(() => setLoaded(true)),
  []);

  const refresh = useCallback(() => {
    setLoaded(false);
    return load();
  }, [load]);

  useEffect(() => {
    void load();
  }, [load]);

  return { measurementTypes, loaded, refresh };
}

export function useMeasurementTypesWithReadings() {
  const [measurementTypes, setMeasurementTypes] = useState<MeasurementTypeInfo[]>([]);
  const [loaded, setLoaded] = useState(false);

  const load = useCallback(() =>
    apiClient.GET('/measurement-types', { params: { query: { has_readings: true } } })
      .then(({ data }) => setMeasurementTypes(data ?? []))
      .catch((err) => logger.error('Failed to fetch measurement types with readings:', err))
      .finally(() => setLoaded(true)),
  []);

  const refresh = useCallback(() => {
    setLoaded(false);
    return load();
  }, [load]);

  useEffect(() => {
    void load();
  }, [load]);

  return { measurementTypes, loaded, refresh };
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
