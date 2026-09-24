import { createContext, useContext, useLayoutEffect } from 'react';

export const InsetContext = createContext(false);

export const BleedContext = createContext<(() => () => void) | null>(null);

export function useInsetApplied() {
  return useContext(InsetContext);
}

export function useBleed(active: boolean) {
  const register = useContext(BleedContext);
  useLayoutEffect(() => (active && register ? register() : undefined), [active, register]);
}
