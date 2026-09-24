import { describe, expect, it } from 'vitest';
import type { WidgetDefinition } from './types';
import { getAllWidgets } from './WidgetRegistry';
import { registerAllWidgets } from './widgets';

registerAllWidgets();

describe('widget registry', () => {
  it('gives every widget type its compact height', () => {
    const heights = Object.fromEntries(getAllWidgets().map((widget) => [widget.type, widget.compactHeight]));

    expect(heights).toEqual({
      'readings-chart': 280,
      'comparison-chart': 280,
      heatmap: 260,
      gauge: 200,
      'sensor-health-pie': 220,
      'sensor-type-pie': 220,
      'min-max-avg': 160,
      'current-reading': 140,
      uptime: 140,
      'sensor-toggle': 120,
      'health-timeline': 220,
      'markdown-note': 'content',
      'alert-summary': 'content',
      'notifications-feed': 'content',
      'group-summary': 'content',
      'live-readings': 'content',
      'reading-stats': 'content',
      'weather-forecast': 'content',
      'sensor-detail': 'content',
    });
  });

  it('requires a compact height on every definition', () => {
    // @ts-expect-error compactHeight is required
    const definition: WidgetDefinition = {
      type: 'no-height',
      label: 'No height',
      description: '',
      kind: 'informational',
      component: () => null,
      defaultConfig: {},
      defaultLayout: { w: 1, h: 1 },
    };
    expect(definition.compactHeight).toBeUndefined();
  });
});
