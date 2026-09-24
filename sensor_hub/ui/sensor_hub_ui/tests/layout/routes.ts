import type { LayoutUser } from './users';

export const layoutChecks = ['noSidewaysScroll', 'noCollapsedContent', 'shell', 'appBar'] as const;
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
  { path: '/dashboard', users: signedIn, fixtures: ['dashboard'], checks: ['shell', 'appBar'] },
  { path: '/sensors-overview', users: signedIn, fixtures: [], checks: ['shell', 'appBar'] },
  { path: '/sensor/1', users: signedIn, fixtures: [], checks: ['shell', 'appBar'] },
  { path: '/properties-overview', users: signedIn, fixtures: [], checks: ['shell', 'appBar'] },
  { path: '/data-retention', users: signedIn, fixtures: [], checks: ['shell', 'appBar'] },
  { path: '/notifications', users: signedIn, fixtures: [], checks: ['shell', 'appBar'] },
  { path: '/mqtt', users: signedIn, fixtures: [], checks: ['shell', 'appBar'] },
  { path: '/admin', users: signedIn, fixtures: [], checks: ['shell', 'appBar'] },
  { path: '/account/sessions', users: signedIn, fixtures: [], checks: ['shell', 'appBar'] },
  { path: '/account/change-password', users: signedIn, fixtures: [], checks: ['shell', 'appBar'] },
  { path: '/account/developer', users: signedIn, fixtures: [], checks: ['shell', 'appBar'] },
];
