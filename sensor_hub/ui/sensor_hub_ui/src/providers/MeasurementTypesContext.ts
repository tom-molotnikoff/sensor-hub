import { createContext } from 'react';
import type { MeasurementTypeInfo } from '../gen/aliases';

export type MeasurementTypeListName = 'all' | 'withReadings';

export type MeasurementTypeLists = Record<MeasurementTypeListName, MeasurementTypeInfo[]>;

export interface MeasurementTypesContextValue {
  lists: MeasurementTypeLists;
  request: (list: MeasurementTypeListName) => void;
}

const NO_TYPES: MeasurementTypeInfo[] = [];

export const MeasurementTypesContext = createContext<MeasurementTypesContextValue>({
  lists: { all: NO_TYPES, withReadings: NO_TYPES },
  request: () => {},
});
