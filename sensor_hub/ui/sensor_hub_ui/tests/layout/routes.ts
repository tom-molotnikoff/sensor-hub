import type { LayoutUser } from './users';

export const layoutChecks = ['noSidewaysScroll', 'noCollapsedContent'] as const;
export type LayoutCheck = (typeof layoutChecks)[number];

export interface LayoutRoute {
  path: string;
  users: readonly LayoutUser[];
  fixtures: readonly string[];
  checks?: readonly LayoutCheck[];
}

export const routes: readonly LayoutRoute[] = [
  { path: '/login', users: ['anonymous'], fixtures: [] },
];
