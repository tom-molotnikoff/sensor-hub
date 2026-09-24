import { expect, test } from './test';
import { credentials, signIn, type SignedInUser } from './users';

const roles: Record<SignedInUser, string> = { admin: 'admin', viewer: 'viewer' };

for (const user of Object.keys(credentials) as SignedInUser[]) {
  test(`${user} signs in through the login endpoint`, async ({ page }) => {
    await signIn(page, user);

    const me = await page.request.get('/api/auth/me');
    expect(me.status()).toBe(200);
    const { user: current } = await me.json();
    expect(current.username).toBe(credentials[user].username);
    expect(current.roles).toEqual([roles[user]]);
    expect(current.must_change_password).toBe(false);

    await page.goto('/sensors-overview');
    await expect(page).toHaveURL(/\/sensors-overview$/);
  });
}
