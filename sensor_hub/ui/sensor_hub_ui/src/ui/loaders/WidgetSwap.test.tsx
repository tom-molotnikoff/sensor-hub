import { act, render, screen } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import WidgetSwap from './WidgetSwap';

function renderSwap(loading: boolean) {
  return render(
    <WidgetSwap loading={loading} loader={<span data-testid="widget-loader">loader</span>}>
      <span>No data available</span>
    </WidgetSwap>,
  );
}

describe('WidgetSwap', () => {
  beforeEach(() => {
    vi.useFakeTimers();
  });

  afterEach(() => {
    vi.runOnlyPendingTimers();
    vi.useRealTimers();
  });

  it('shows neither the loader nor the widget content in the first 100 ms of a load', () => {
    renderSwap(true);

    expect(screen.queryByTestId('widget-loader')).not.toBeInTheDocument();
    expect(screen.queryByText('No data available')).not.toBeInTheDocument();
  });

  it('shows the loader once the load outlasts 100 ms', () => {
    renderSwap(true);
    act(() => { vi.advanceTimersByTime(100); });

    expect(screen.getByTestId('widget-loader')).toBeInTheDocument();
    expect(screen.queryByText('No data available')).not.toBeInTheDocument();
  });

  it('goes straight to the content when the load finishes inside the window', () => {
    const { rerender } = renderSwap(true);
    act(() => { vi.advanceTimersByTime(50); });

    rerender(
      <WidgetSwap loading={false} loader={<span data-testid="widget-loader">loader</span>}>
        <span>No data available</span>
      </WidgetSwap>,
    );

    expect(screen.getByText('No data available')).toBeInTheDocument();
    expect(screen.queryByTestId('widget-loader')).not.toBeInTheDocument();
  });
});
