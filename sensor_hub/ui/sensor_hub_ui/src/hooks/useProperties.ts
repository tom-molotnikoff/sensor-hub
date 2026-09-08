import { useContext } from 'react';
import { PropertiesContext } from '../providers/PropertiesContext';

export function useProperties(): Record<string, string> {
  return useContext(PropertiesContext);
}
