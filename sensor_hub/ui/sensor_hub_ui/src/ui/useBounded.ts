import { createContext, useContext } from 'react';

export const BoundedContext = createContext(false);

export function useBounded() {
  return useContext(BoundedContext);
}
