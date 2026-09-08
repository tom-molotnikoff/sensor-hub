import { createContext } from 'react';
import type { CommandStatusMessage, Reading } from '../gen/aliases';

export type CurrentReadingsMap = Record<string, Record<string, Reading>>;

export interface CurrentReadingsListeners {
  onDataUpdate?: (date: Date) => void;
  onCommandStatus?: (message: CommandStatusMessage) => void;
}

export interface CurrentReadingsContextValue {
  readings: CurrentReadingsMap;
  ready: boolean;
  subscribe: (listeners: CurrentReadingsListeners) => () => void;
}

const NO_READINGS: CurrentReadingsMap = {};

export const CurrentReadingsContext = createContext<CurrentReadingsContextValue>({
  readings: NO_READINGS,
  ready: false,
  subscribe: () => () => {},
});
