import type { ReactNode } from 'react';
import { BoundedContext } from './useBounded';

export default function Bounded({ children }: { children?: ReactNode }) {
  return <BoundedContext.Provider value={true}>{children}</BoundedContext.Provider>;
}
