import { useContext, useEffect, useRef } from 'react';
import {
  CurrentReadingsContext,
  type CurrentReadingsListeners,
  type CurrentReadingsMap,
} from '../providers/CurrentReadingsContext';

export type { CurrentReadingsMap };

export function useCurrentReadingsReady(): boolean {
  return useContext(CurrentReadingsContext).ready;
}

export function useCurrentReadings(options?: CurrentReadingsListeners): CurrentReadingsMap {
  const { readings, subscribe } = useContext(CurrentReadingsContext);

  const optionsRef = useRef(options);
  useEffect(() => {
    optionsRef.current = options;
  });

  useEffect(() => subscribe({
    onDataUpdate: (at) => optionsRef.current?.onDataUpdate?.(at),
    onCommandStatus: (message) => optionsRef.current?.onCommandStatus?.(message),
  }), [subscribe]);

  return readings;
}
