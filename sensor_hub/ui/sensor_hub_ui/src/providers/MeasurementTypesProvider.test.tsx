import { renderHook, waitFor } from '@testing-library/react';
import type { ReactNode } from 'react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import type { MeasurementTypeInfo } from '../gen/aliases';
import { useMeasurementTypes, useMeasurementTypesWithReadings } from '../hooks/useMeasurementTypes';
import { requestScheduler } from '../scheduler/requestScheduler';
import MeasurementTypesProvider from './MeasurementTypesProvider';

const { getMock } = vi.hoisted(() => ({ getMock: vi.fn() }));

vi.mock('../gen/client', () => ({
  apiClient: { GET: getMock },
}));

const temperature: MeasurementTypeInfo = {
  id: 1,
  name: 'temperature',
  display_name: 'Temperature',
  unit: '°C',
  category: 'numeric',
  default_aggregation_function: 'avg',
  supported_aggregation_functions: ['avg', 'min', 'max'],
};

function wrapper({ children }: { children: ReactNode }) {
  return <MeasurementTypesProvider>{children}</MeasurementTypesProvider>;
}

function queriesFor(hasReadings: boolean | undefined) {
  return getMock.mock.calls.filter(
    ([path, init]) => path === '/measurement-types'
      && (init?.params?.query?.has_readings ?? undefined) === hasReadings,
  );
}

describe('MeasurementTypesProvider', () => {
  beforeEach(() => {
    getMock.mockReset();
    getMock.mockResolvedValue({ data: [temperature] });
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it('asks for nothing while no consumer needs a list', () => {
    renderHook(() => null, { wrapper });

    expect(getMock).not.toHaveBeenCalled();
  });

  it('leaves the types with readings unrequested until a consumer is enabled', async () => {
    const scheduleSpy = vi.spyOn(requestScheduler, 'schedule');
    const { result, rerender } = renderHook(
      ({ open }) => useMeasurementTypesWithReadings(open),
      { wrapper, initialProps: { open: false } },
    );

    expect(queriesFor(true)).toHaveLength(0);

    rerender({ open: true });
    await waitFor(() => expect(result.current).toEqual([temperature]));

    expect(queriesFor(true)).toHaveLength(1);
    expect(scheduleSpy).toHaveBeenCalledWith('normal', expect.any(Function));
  });

  it('serves a reopened consumer from the provider without asking again', async () => {
    const { result, rerender } = renderHook(
      ({ open }) => useMeasurementTypesWithReadings(open),
      { wrapper, initialProps: { open: true } },
    );

    await waitFor(() => expect(result.current).toEqual([temperature]));

    rerender({ open: false });
    rerender({ open: true });

    expect(result.current).toEqual([temperature]);
    expect(queriesFor(true)).toHaveLength(1);
  });

  it('requests the plain list once however many consumers ask for it', async () => {
    const { result } = renderHook(
      () => [useMeasurementTypes(), useMeasurementTypes(), useMeasurementTypes()] as const,
      { wrapper },
    );

    await waitFor(() => expect(result.current[0]).toEqual([temperature]));

    expect(queriesFor(undefined)).toHaveLength(1);
    expect(result.current[1]).toBe(result.current[0]);
    expect(result.current[2]).toBe(result.current[0]);
  });

  it('keeps the two lists apart', async () => {
    getMock.mockImplementation((_path: string, init?: { params?: { query?: { has_readings?: boolean } } }) => (
      Promise.resolve({ data: init?.params?.query?.has_readings ? [temperature] : [] })
    ));

    const { result } = renderHook(
      () => ({ all: useMeasurementTypes(), withReadings: useMeasurementTypesWithReadings() }),
      { wrapper },
    );

    await waitFor(() => expect(result.current.withReadings).toEqual([temperature]));
    expect(result.current.all).toEqual([]);
  });
});
