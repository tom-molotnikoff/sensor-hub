import { useQuery } from '@tanstack/react-query';
import { unwrap } from '../api/unwrap';
import { apiClient } from '../gen/client';

export function useSensorCommands(sensorId: number) {
  return useQuery({
    queryKey: ['sensors', sensorId, 'commands'],
    queryFn: () => unwrap(apiClient.GET('/sensors/by-id/{id}/commands', { params: { path: { id: sensorId } } })),
    refetchInterval: 30_000,
  });
}
