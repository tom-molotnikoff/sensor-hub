import type { Theme } from '@mui/material';
import type { CSSObject } from '@mui/system';

const APP_BAR_HEIGHT = 64;
const LANDING_GAP = 24;
const RAIL_GAP = 16;

export function landingOffset(headerHeight: number): number {
  return APP_BAR_HEIGHT + headerHeight + LANDING_GAP;
}

export function railOffset(headerHeight: number): number {
  return APP_BAR_HEIGHT + headerHeight + RAIL_GAP;
}

export function belowAppBar(theme: Theme): CSSObject {
  const rules: CSSObject = {};
  for (const [key, value] of Object.entries(theme.mixins.toolbar)) {
    if (key === 'minHeight') rules.top = value as CSSObject['top'];
    else if (typeof value === 'object' && value !== null && 'minHeight' in value) {
      rules[key] = { top: (value as { minHeight: number }).minHeight };
    }
  }
  return rules;
}
