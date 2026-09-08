import { type ReactNode, useCallback, useMemo, useRef, useState } from 'react';
import type { CommandStatusMessage, Reading } from '../gen/aliases';
import { WEBSOCKET_BASE } from '../environment/Environment';
import { useReconnectingWebSocket } from '../hooks/useReconnectingWebSocket';
import { logger } from '../tools/logger';
import { useAuth } from './AuthContext';
import {
  CurrentReadingsContext,
  type CurrentReadingsListeners,
  type CurrentReadingsMap,
} from './CurrentReadingsContext';

interface CurrentReadingsProviderProps {
  children: ReactNode;
}

function mergeReadings(prev: CurrentReadingsMap, readings: Reading[]): CurrentReadingsMap {
  const next: CurrentReadingsMap = { ...prev };
  readings.forEach((reading) => {
    if (!reading) return;
    const sensorEntry = next[reading.sensor_name]
      ? { ...next[reading.sensor_name] }
      : {};
    sensorEntry[reading.measurement_type] = reading;
    next[reading.sensor_name] = sensorEntry;
  });
  return next;
}

export default function CurrentReadingsProvider({ children }: CurrentReadingsProviderProps) {
  const [readings, setReadings] = useState<CurrentReadingsMap>({});
  const [ready, setReady] = useState(false);
  const { user } = useAuth();
  const listenersRef = useRef(new Set<CurrentReadingsListeners>());

  const subscribe = useCallback((listeners: CurrentReadingsListeners) => {
    const registered = listenersRef.current;
    registered.add(listeners);
    return () => {
      registered.delete(listeners);
    };
  }, []);

  const applyReadings = useCallback((incoming: Reading[]) => {
    setReadings((prev) => mergeReadings(prev, incoming));
    setReady(true);
    const at = new Date();
    listenersRef.current.forEach((listeners) => listeners.onDataUpdate?.(at));
  }, []);

  const handleMessage = useCallback((event: MessageEvent) => {
    if (!event.data || event.data === 'null') return;

    let parsed: unknown;
    try {
      parsed = JSON.parse(event.data);
    } catch (e) {
      logger.error('Readings WS: failed to parse message', e, event.data);
      return;
    }

    if (Array.isArray(parsed)) {
      applyReadings(parsed as Reading[]);
      return;
    }

    if (typeof parsed !== 'object' || parsed === null) return;

    if ('type' in parsed && parsed.type === 'command_status') {
      const message = parsed as CommandStatusMessage;
      listenersRef.current.forEach((listeners) => listeners.onCommandStatus?.(message));
      return;
    }

    if ('readings' in parsed && Array.isArray(parsed.readings)) {
      applyReadings(parsed.readings as Reading[]);
      return;
    }

    if ('sensor_name' in parsed && 'measurement_type' in parsed) {
      applyReadings([parsed as Reading]);
    }
  }, [applyReadings]);

  useReconnectingWebSocket({
    url: `${WEBSOCKET_BASE}/readings/ws/current`,
    onMessage: handleMessage,
    enabled: user != null,
  });

  const value = useMemo(
    () => ({ readings, ready, subscribe }),
    [readings, ready, subscribe],
  );

  return (
    <CurrentReadingsContext.Provider value={value}>
      {children}
    </CurrentReadingsContext.Provider>
  );
}
