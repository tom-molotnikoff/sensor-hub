import type { LayoutUser } from './users';

export const layoutChecks = ['noSidewaysScroll', 'noCollapsedContent', 'shell'] as const;
export type LayoutCheck = (typeof layoutChecks)[number];

export interface LayoutRoute {
  path: string;
  users: readonly LayoutUser[];
  fixtures: readonly string[];
  checks?: readonly LayoutCheck[];
}

const signedIn = ['admin', 'viewer'] as const;

export const routes: readonly LayoutRoute[] = [
  { path: '/login', users: ['anonymous'], fixtures: [], checks: ['noSidewaysScroll', 'noCollapsedContent'] },
  { path: '/dashboard', users: signedIn, fixtures: [], checks: ['shell'] },
  { path: '/sensors-overview', users: signedIn, fixtures: [], checks: ['shell'] },
  { path: '/sensor/1', users: signedIn, fixtures: [], checks: ['shell'] },
  { path: '/properties-overview', users: signedIn, fixtures: [], checks: ['shell'] },
  { path: '/data-retention', users: signedIn, fixtures: [], checks: ['shell'] },
  { path: '/notifications', users: signedIn, fixtures: [], checks: ['shell'] },
  { path: '/mqtt', users: signedIn, fixtures: [], checks: ['shell'] },
  { path: '/admin', users: signedIn, fixtures: [], checks: ['shell'] },
  { path: '/account/sessions', users: signedIn, fixtures: [], checks: ['shell'] },
  { path: '/account/change-password', users: signedIn, fixtures: [], checks: ['shell'] },
  { path: '/account/developer', users: signedIn, fixtures: [], checks: ['shell'] },
];
