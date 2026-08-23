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

export function useSensorMeasurementTypes(sensorId: number | null) {
  const [measurementTypes, setMeasurementTypes] = useState<MeasurementTypeInfo[]>([]);
  const [loaded, setLoaded] = useState(false);

  const load = useCallback(() => {
    if (sensorId === null) {
      return Promise.resolve().then(() => {
        setMeasurementTypes([]);
        setLoaded(true);
      });
    }
    return apiClient.GET('/sensors/by-id/{id}/measurement-types', {
      params: { path: { id: sensorId } },
    })
      .then(({ data }) => setMeasurementTypes(data ?? []))
      .catch((err) => logger.error('Failed to fetch sensor measurement types:', err))
      .finally(() => setLoaded(true));
  }, [sensorId]);

  const refresh = useCallback(() => {
    setLoaded(false);
    return load();
  }, [load]);

  useEffect(() => {
    void load();
  }, [load]);

  return { measurementTypes, loaded, refresh };
}
