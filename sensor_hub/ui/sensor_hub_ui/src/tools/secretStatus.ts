// A stored secret in needs_reentry does not decrypt under the hub's current
// key, such as after the key was replaced, so whatever uses it stays off until
// it is entered again.
export const NEEDS_REENTRY_LABEL = 'Needs re-entry';
export const NEEDS_REENTRY_TEXT = 'This secret could not be decrypted. Enter it again.';

export function needsReentry(owner: { password_status?: string }): boolean {
  return owner.password_status === 'needs_reentry';
}
