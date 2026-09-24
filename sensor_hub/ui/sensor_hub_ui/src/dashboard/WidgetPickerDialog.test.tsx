import { fireEvent, render, screen } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';
import type { DashboardWidget } from '../gen/aliases';
import { DashboardContext, type DashboardContextValue } from './DashboardContext';
import WidgetPickerDialog from './WidgetPickerDialog';
import { registerAllWidgets } from './widgets';

registerAllWidgets();

describe('WidgetPickerDialog', () => {
  it('places a new widget at x=0 below the lowest widget, at its default size', () => {
    const addWidget = vi.fn();
    const picker = (widgets: DashboardWidget[]) => (
      <DashboardContext.Provider value={{ config: { widgets }, addWidget } as unknown as DashboardContextValue}>
        <WidgetPickerDialog open onClose={() => {}} />
      </DashboardContext.Provider>
    );
    const { rerender } = render(picker([]));
    fireEvent.click(screen.getByText('Gauge'));

    rerender(
      picker([
        { id: 'a', type: 'uptime', config: {}, layout: { x: 0, y: 0, w: 6, h: 5 } },
        { id: 'b', type: 'uptime', config: {}, layout: { x: 6, y: 2, w: 6, h: 4 } },
      ]),
    );
    fireEvent.click(screen.getByText('Gauge'));

    const placed = addWidget.mock.calls.map(([widget]) => (widget as DashboardWidget).layout);
    expect(placed).toEqual([
      { x: 0, y: 0, w: 3, h: 3 },
      { x: 0, y: 6, w: 3, h: 3 },
    ]);
  });
});
