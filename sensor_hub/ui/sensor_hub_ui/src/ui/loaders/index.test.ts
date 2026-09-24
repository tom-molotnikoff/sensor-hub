import { describe, expect, it } from 'vitest';
import * as loaders from './index';

/**
 * Coverage guard for the Incoming loader family. Every widget shape that gets a
 * loader must have its component exported here, so the family stays complete and
 * discoverable. If you add a new loader, add it to this list.
 */
const EXPECTED_LOADERS = [
  'ValuePlaceholderLoader',
  'SignalTraceLoader',
  'CircularDrawLoader',
  'RippleHeatmapLoader',
  'CascadeRowsLoader',
  'ScanLineLoader',
  'SkeletonTilesLoader',
  'IndeterminateBarLoader',
  'WeatherColumnsLoader',
  'SensorDetailTilesLoader',
];

describe('widget-loaders family', () => {
  it('exports every loader plus the shared primitives', () => {
    for (const name of EXPECTED_LOADERS) {
      expect(loaders[name as keyof typeof loaders], `missing loader export: ${name}`).toBeTypeOf('function');
    }
    expect(loaders.WidgetSwap).toBeTypeOf('function');
    expect(loaders.LoaderShell).toBeTypeOf('function');
    expect(loaders.useLoaderVisibility).toBeTypeOf('function');
    expect(loaders.usePrefersReducedMotion).toBeTypeOf('function');
  });
});
