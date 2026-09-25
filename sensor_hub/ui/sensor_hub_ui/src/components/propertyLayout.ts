const LANDING_GAP = 24;
const RAIL_GAP = 16;

export function landingOffset(stickyTop: number, headerHeight: number): number {
  return stickyTop + headerHeight + LANDING_GAP;
}

export function railOffset(headerHeight: number): number {
  return headerHeight + RAIL_GAP;
}
