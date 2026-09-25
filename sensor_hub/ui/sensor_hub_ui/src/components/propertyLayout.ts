import type { Tier, TierValues } from '../ui/tiers';

const APP_BAR_HEIGHT: TierValues<number> = { compact: 64, wide: 0 };
const LANDING_GAP = 24;
const RAIL_GAP = 16;

export function landingOffset(headerHeight: number, tier: Tier): number {
  return APP_BAR_HEIGHT[tier] + headerHeight + LANDING_GAP;
}

export function railOffset(headerHeight: number): number {
  return headerHeight + RAIL_GAP;
}
