import { act, renderHook } from '@testing-library/react';
import type { ReactNode } from 'react';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import type { CommandStatusMessage } from '../gen/aliases';
import { AuthContext } from '../providers/AuthContext';
import CurrentReadingsProvider from '../providers/CurrentReadingsProvider';
import { useCurrentReadings, useCurrentReadingsReady } from './useCurrentReadings';

let socketMessageHandler: ((event: MessageEvent) => void) | undefined;

vi.mock('./useReconnectingWebSocket', () => ({
  useReconnectingWebSocket: (options: { onMessage: (event: MessageEvent) => void }) => {
    socketMessageHandler = options.onMessage;
  },
}));

const user = { id: 1, username: 'operator', roles: [], permissions: ['view_readings'] };

function wrapper({ children }: { children: ReactNode }) {
  return (
    <AuthContext.Provider value={{ user, refresh: async () => {} }}>
      <CurrentReadingsProvider>{children}</CurrentReadingsProvider>
    </AuthContext.Provider>
  );
}

function snapshot(textState: string) {
  return new MessageEvent('message', {
    data: JSON.stringify([{
      id: 99,
      sensor_name: 'office-plug',
      measurement_type: 'state',
      numeric_value: null,
      text_state: textState,
      unit: '',
      time: '2026-05-09T20:00:00Z',
    }]),
  });
}

describe('useCurrentReadings', () => {
  beforeEach(() => {
    socketMessageHandler = undefined;
  });

  it('holds no readings and reports not ready until the snapshot arrives', () => {
    const { result } = renderHook(
      () => ({ readings: useCurrentReadings(), ready: useCurrentReadingsReady() }),
      { wrapper },
    );

    expect(result.current.readings).toEqual({});
    expect(result.current.ready).toBe(false);

    act(() => {
      socketMessageHandler?.(snapshot('ON'));
    });

    expect(result.current.readings['office-plug']?.state?.text_state).toBe('ON');
    expect(result.current.ready).toBe(true);
  });

  it('delivers the snapshot to every consumer under the one provider', () => {
    const { result } = renderHook(
      () => [useCurrentReadings(), useCurrentReadings()] as const,
      { wrapper },
    );

    act(() => {
      socketMessageHandler?.(snapshot('OFF'));
    });

    expect(result.current[0]['office-plug']?.state?.text_state).toBe('OFF');
    expect(result.current[1]).toBe(result.current[0]);
  });

  it('merges an incremental reading into the readings already held', () => {
    const { result } = renderHook(() => useCurrentReadings(), { wrapper });

    act(() => {
      socketMessageHandler?.(snapshot('ON'));
    });
    act(() => {
      socketMessageHandler?.(new MessageEvent('message', {
        data: JSON.stringify({
          id: 100,
          sensor_name: 'office-plug',
          measurement_type: 'power',
          numeric_value: 12.5,
          text_state: null,
          unit: 'W',
          time: '2026-05-09T20:01:00Z',
        }),
      }));
    });

    expect(result.current['office-plug']?.state?.text_state).toBe('ON');
    expect(result.current['office-plug']?.power?.numeric_value).toBe(12.5);
  });

  it('reports data updates and command status to the callbacks it was given', () => {
    const onDataUpdate = vi.fn();
    const onCommandStatus = vi.fn();
    renderHook(() => useCurrentReadings({ onDataUpdate, onCommandStatus }), { wrapper });

    act(() => {
      socketMessageHandler?.(snapshot('ON'));
    });

    expect(onDataUpdate).toHaveBeenCalledTimes(1);
    expect(onDataUpdate.mock.calls[0][0]).toBeInstanceOf(Date);

    const message: CommandStatusMessage = {
      type: 'command_status',
      id: 4,
      sensor_id: 7,
      property: 'state',
      value: 'ON',
      status: 'failed',
    };
    act(() => {
      socketMessageHandler?.(new MessageEvent('message', { data: JSON.stringify(message) }));
    });

    expect(onCommandStatus).toHaveBeenCalledWith(message);
    expect(onDataUpdate).toHaveBeenCalledTimes(1);
  });

  it('stops calling the callbacks of a consumer that has unmounted', () => {
    const onDataUpdate = vi.fn();
    const { unmount } = renderHook(() => useCurrentReadings({ onDataUpdate }), { wrapper });

    unmount();
    act(() => {
      socketMessageHandler?.(snapshot('ON'));
    });

    expect(onDataUpdate).not.toHaveBeenCalled();
  });
});
