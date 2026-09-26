import type { LayoutUser } from './users';

export const layoutChecks = ['noSidewaysScroll', 'noCollapsedContent', 'shell', 'appBar', 'pinnedTitle', 'scrollingTitle'] as const;
export type LayoutCheck = (typeof layoutChecks)[number];

const pageChecks = ['noSidewaysScroll', 'noCollapsedContent', 'shell', 'appBar'] as const satisfies readonly LayoutCheck[];

export const defaultChecks: readonly LayoutCheck[] = [...pageChecks, 'scrollingTitle'];

const pinnedHeaderChecks: readonly LayoutCheck[] = [...pageChecks, 'pinnedTitle'];

export interface LayoutRoute {
  path: string;
  users: readonly LayoutUser[];
  fixtures: readonly string[];
  checks?: readonly LayoutCheck[];
}

const signedIn = ['admin', 'viewer'] as const;

export const routes: readonly LayoutRoute[] = [
  { path: '/login', users: ['anonymous'], fixtures: [], checks: ['noSidewaysScroll', 'noCollapsedContent'] },
  { path: '/dashboard', users: signedIn, fixtures: ['sensors', 'dashboard', 'health-history', 'current-readings', 'alerts', 'notifications'] },
  { path: '/sensors-overview', users: signedIn, fixtures: ['sensors', 'pending-sensors'] },
  { path: '/sensor/1', users: signedIn, fixtures: ['health-history'] },
  { path: '/sensor/9', users: signedIn, fixtures: ['sensors', 'health-history'] },
  { path: '/properties-overview', users: signedIn, fixtures: [], checks: pinnedHeaderChecks },
  { path: '/data-retention', users: signedIn, fixtures: ['sensors'] },
  { path: '/notifications', users: signedIn, fixtures: ['alerts', 'notifications'] },
  { path: '/mqtt', users: signedIn, fixtures: ['pending-sensors', 'mqtt'] },
  { path: '/admin', users: signedIn, fixtures: ['users'] },
  { path: '/account/sessions', users: signedIn, fixtures: ['sessions'] },
  { path: '/account/change-password', users: signedIn, fixtures: [] },
  { path: '/account/developer', users: signedIn, fixtures: ['api-keys'] },
];
