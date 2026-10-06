import { useCallback } from 'react';
import { useSensorContext } from '../../hooks/useSensorContext';

export function useSensorName() {
  const { sensors } = useSensorContext();
  return useCallback((id: number) => sensors.find((sensor) => sensor.id === id)?.name ?? `sensor ${id}`, [sensors]);
}
