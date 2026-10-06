import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import type { Automation, AutomationInput } from '../gen/aliases';
import { unwrap } from '../api/unwrap';
import { apiClient } from '../gen/client';

// Statuses and next fire times move on as the scheduler fires, with nothing pushed to the page.
const refetchInterval = 30_000;

const keys = {
  all: ['automations'] as const,
  one: (id: number) => ['automations', id] as const,
  runs: (id: number) => ['automations', id, 'runs'] as const,
};

export function useAutomations() {
  return useQuery({
    queryKey: keys.all,
    queryFn: () => unwrap(apiClient.GET('/automations')),
    refetchInterval,
  });
}

export function useAutomation(id: number | undefined) {
  return useQuery({
    queryKey: keys.one(id ?? 0),
    queryFn: () => unwrap(apiClient.GET('/automations/{id}', { params: { path: { id: id! } } })),
    enabled: id !== undefined,
    refetchInterval,
  });
}

export function useAutomationRuns(id: number | undefined) {
  return useQuery({
    queryKey: keys.runs(id ?? 0),
    queryFn: () => unwrap(apiClient.GET('/automations/{id}/runs', { params: { path: { id: id! } } })),
    enabled: id !== undefined,
    refetchInterval,
  });
}

export function useSaveAutomation() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ id, input }: { id?: number; input: AutomationInput }) =>
      id === undefined
        ? unwrap(apiClient.POST('/automations', { body: input }))
        : unwrap(apiClient.PUT('/automations/{id}', { params: { path: { id } }, body: input })),
    onSuccess: (saved) => {
      queryClient.setQueryData(keys.one(saved.id), saved);
      return queryClient.invalidateQueries({ queryKey: keys.all });
    },
  });
}

export function useSetAutomationEnabled() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ id, enabled }: { id: number; enabled: boolean }) =>
      unwrap(apiClient.PUT('/automations/{id}/enabled', { params: { path: { id } }, body: { enabled } })),
    onSuccess: (saved: Automation) => {
      queryClient.setQueryData(keys.one(saved.id), saved);
      return queryClient.invalidateQueries({ queryKey: keys.all });
    },
  });
}

export function useRunAutomation() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (id: number) => unwrap(apiClient.POST('/automations/{id}/run', { params: { path: { id } } })),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: keys.all }),
  });
}

export function useCancelAutomationRun() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ id, runId }: { id: number; runId: number }) =>
      unwrap(apiClient.POST('/automations/{id}/runs/{runId}/cancel', { params: { path: { id, runId } } })),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: keys.all }),
  });
}

export function useDeleteAutomation() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (id: number) => unwrap(apiClient.DELETE('/automations/{id}', { params: { path: { id } } })),
    onSuccess: (_, id) => {
      queryClient.removeQueries({ queryKey: keys.one(id) });
      return queryClient.invalidateQueries({ queryKey: keys.all, exact: true });
    },
  });
}
