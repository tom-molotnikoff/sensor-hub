import { expect, type Page } from '@playwright/test';

export const credentials = {
  admin: { username: 'testadmin', password: 'testpassword123' },
  viewer: { username: 'testviewer', password: 'viewerpassword123' },
} as const;

export type SignedInUser = keyof typeof credentials;
export type LayoutUser = SignedInUser | 'anonymous';

export async function signIn(page: Page, user: LayoutUser): Promise<string | undefined> {
  if (user === 'anonymous') return undefined;
  const response = await page.request.post('/api/auth/login', { data: credentials[user] });
  expect(response.status(), `sign in as ${user}`).toBe(200);
  return ((await response.json()) as { csrf_token: string }).csrf_token;
}
