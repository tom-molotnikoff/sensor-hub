import { act, render } from '@testing-library/react';
import type { ReactNode } from 'react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import {
    WidgetStateReportContext,
    WidgetViewportContext,
    type WidgetKind,
    type WidgetState,
} from '../dashboard/WidgetContext';
import { requestScheduler } from '../scheduler/requestScheduler';
import { useScheduledQuery, type ScheduledQueryResult } from './useScheduledQuery';

interface HarnessProps {
    visible: boolean;
    kind?: WidgetKind;
    children: ReactNode;
}

function Harness({ visible, kind = 'informational', children }: HarnessProps) {
    return (
        <WidgetViewportContext.Provider value={{ visible, kind }}>
            {children}
        </WidgetViewportContext.Provider>
    );
}

function setTabHidden(hidden: boolean): void {
    Object.defineProperty(document, 'visibilityState', {
        configurable: true,
        get: () => (hidden ? 'hidden' : 'visible'),
    });
    document.dispatchEvent(new Event('visibilitychange'));
}

async function settle(): Promise<void> {
    await act(async () => {
        await Promise.resolve();
        await Promise.resolve();
        await Promise.resolve();
    });
}

async function tick(ms: number): Promise<void> {
    await act(async () => {
        vi.advanceTimersByTime(ms);
        await Promise.resolve();
        await Promise.resolve();
    });
}

type Options = Parameters<typeof useScheduledQuery>[1];

function renderQuery<T>(
    fetcher: (signal: AbortSignal) => Promise<T>,
    options: Options,
    initial: { visible: boolean; kind?: WidgetKind },
) {
    const seen: ScheduledQueryResult<T>[] = [];
    const states: WidgetState[] = [];

    function Probe() {
        seen.push(useScheduledQuery(fetcher, options));
        return null;
    }

    function Tree({ visible, kind }: { visible: boolean; kind?: WidgetKind }) {
        return (
            <Harness visible={visible} kind={kind}>
                <WidgetStateReportContext.Provider value={(state) => states.push(state)}>
                    <Probe />
                </WidgetStateReportContext.Provider>
            </Harness>
        );
    }

    const view = render(<Tree {...initial} />);
    return {
        states,
        latest: () => seen[seen.length - 1],
        setVisible: (visible: boolean) => view.rerender(<Tree {...initial} visible={visible} />),
        unmount: view.unmount,
    };
}

describe('useScheduledQuery', () => {
    beforeEach(() => {
        vi.useFakeTimers();
        setTabHidden(false);
    });

    afterEach(() => {
        vi.useRealTimers();
        requestScheduler.setMaxConcurrency(4);
    });

    it('issues no request and registers no poll timer while the frame is held', async () => {
        const setInterval = vi.spyOn(window, 'setInterval');
        const fetcher = vi.fn().mockResolvedValue('rows');

        const query = renderQuery(fetcher, { pollIntervalMs: 30000, deps: [] }, { visible: false });
        await settle();

        expect(fetcher).not.toHaveBeenCalled();
        expect(setInterval).not.toHaveBeenCalled();
        expect(query.latest().status).toBe('held');
        expect(query.states.at(-1)).toBe('held');

        setInterval.mockRestore();
    });

    it('queues the fetch the moment the frame becomes visible', async () => {
        const fetcher = vi.fn().mockResolvedValue('rows');
        const query = renderQuery(fetcher, { deps: [] }, { visible: false });
        await settle();
        expect(fetcher).not.toHaveBeenCalled();

        act(() => query.setVisible(true));
        await settle();

        expect(fetcher).toHaveBeenCalledTimes(1);
        expect(query.latest().data).toBe('rows');
        expect(query.states.at(-1)).toBe('populated');
    });

    it('aborts a queued fetch when the frame leaves view and returns to held with no error', async () => {
        requestScheduler.setMaxConcurrency(1);
        let releaseBlocker!: () => void;
        void requestScheduler.schedule('normal', () => new Promise<void>((resolve) => { releaseBlocker = resolve; }));
        await settle();

        const fetcher = vi.fn().mockResolvedValue('rows');
        const query = renderQuery(fetcher, { deps: [] }, { visible: true });
        await settle();
        expect(query.latest().status).toBe('queued');

        act(() => query.setVisible(false));
        await settle();

        expect(query.latest().status).toBe('held');
        expect(query.latest().error).toBeNull();
        expect(query.states.at(-1)).toBe('held');

        releaseBlocker();
        await settle();
        expect(fetcher).not.toHaveBeenCalled();
    });

    it('reports the frame error when the fetch fails', async () => {
        const fetcher = vi.fn().mockRejectedValue(new Error('boom'));
        const query = renderQuery(fetcher, { deps: [] }, { visible: true });
        await settle();

        expect(query.states.at(-1)).toBe('error');
        expect(query.latest().status).toBe('error');
    });

    it('takes a visible controllable widget at high priority and an informational one at normal', async () => {
        const schedule = vi.spyOn(requestScheduler, 'schedule');
        const fetcher = vi.fn().mockResolvedValue('rows');

        renderQuery(fetcher, { deps: [] }, { visible: true, kind: 'controllable' });
        await settle();
        expect(schedule.mock.calls[0][0]).toBe('high');

        schedule.mockClear();
        renderQuery(fetcher, { deps: [] }, { visible: true });
        await settle();
        expect(schedule.mock.calls[0][0]).toBe('normal');

        schedule.mockRestore();
    });

    it('polls at low priority once the interval has elapsed', async () => {
        const schedule = vi.spyOn(requestScheduler, 'schedule');
        const fetcher = vi.fn().mockResolvedValue('rows');

        renderQuery(fetcher, { pollIntervalMs: 30000, deps: [] }, { visible: true });
        await settle();
        expect(fetcher).toHaveBeenCalledTimes(1);

        await tick(29000);
        expect(fetcher).toHaveBeenCalledTimes(1);

        await tick(1000);
        expect(fetcher).toHaveBeenCalledTimes(2);
        expect(schedule.mock.calls.at(-1)?.[0]).toBe('low');

        schedule.mockRestore();
    });

    it('refreshes on returning to view only once the interval has passed', async () => {
        const fetcher = vi.fn().mockResolvedValue('rows');
        const query = renderQuery(fetcher, { pollIntervalMs: 30000, deps: [] }, { visible: true });
        await settle();
        expect(fetcher).toHaveBeenCalledTimes(1);

        act(() => query.setVisible(false));
        await tick(10000);
        act(() => query.setVisible(true));
        await settle();
        expect(fetcher).toHaveBeenCalledTimes(1);

        act(() => query.setVisible(false));
        await tick(30000);
        act(() => query.setVisible(true));
        await settle();
        expect(fetcher).toHaveBeenCalledTimes(2);
    });

    it('issues no request while the tab is hidden, then one refresh when it is shown again', async () => {
        const fetcher = vi.fn().mockResolvedValue('rows');
        renderQuery(fetcher, { pollIntervalMs: 30000, deps: [] }, { visible: true });
        await settle();
        expect(fetcher).toHaveBeenCalledTimes(1);

        act(() => setTabHidden(true));
        await tick(120000);
        expect(fetcher).toHaveBeenCalledTimes(1);

        await act(async () => {
            setTabHidden(false);
            await Promise.resolve();
            await Promise.resolve();
        });
        expect(fetcher).toHaveBeenCalledTimes(2);
    });

    it('drives twelve visible widgets from one page-level timer', async () => {
        const setInterval = vi.spyOn(window, 'setInterval');
        const fetcher = vi.fn().mockResolvedValue('rows');

        function Probe() {
            useScheduledQuery(fetcher, { pollIntervalMs: 30000, deps: [] });
            return null;
        }

        render(
            <Harness visible>
                {Array.from({ length: 12 }, (_, index) => <Probe key={index} />)}
            </Harness>,
        );
        await settle();
        await settle();

        expect(fetcher).toHaveBeenCalledTimes(12);
        expect(setInterval).toHaveBeenCalledTimes(1);

        setInterval.mockRestore();
    });

    it('never runs more than the scheduler cap of dashboard reads at once', async () => {
        let inFlight = 0;
        let peak = 0;
        const release: (() => void)[] = [];
        const fetcher = vi.fn().mockImplementation(() => {
            inFlight++;
            peak = Math.max(peak, inFlight);
            return new Promise<string>((resolve) => release.push(() => {
                inFlight--;
                resolve('rows');
            }));
        });

        function Probe() {
            useScheduledQuery(fetcher, { deps: [] });
            return null;
        }

        render(
            <Harness visible>
                {Array.from({ length: 12 }, (_, index) => <Probe key={index} />)}
            </Harness>,
        );
        await settle();

        expect(peak).toBe(4);

        while (release.length > 0) {
            release.shift()!();
            await settle();
        }
        expect(peak).toBe(4);
    });

    it('starts again from held when the deps change', async () => {
        const fetcher = vi.fn().mockResolvedValue('rows');
        const seen: string[] = [];

        function Probe({ sensor }: { sensor: string }) {
            const { data, status } = useScheduledQuery(fetcher, { deps: [sensor] });
            seen.push(`${status}:${String(data)}`);
            return null;
        }

        const view = render(<Harness visible><Probe sensor="hall" /></Harness>);
        await settle();
        expect(seen.at(-1)).toBe('fresh:rows');

        const beforeRerender = seen.length;
        view.rerender(<Harness visible><Probe sensor="attic" /></Harness>);
        expect(seen[beforeRerender]).toBe('held:undefined');

        await settle();
        expect(fetcher).toHaveBeenCalledTimes(2);
        expect(seen.at(-1)).toBe('fresh:rows');
    });

    it('stays dormant and reports the frame populated while disabled', async () => {
        const fetcher = vi.fn().mockResolvedValue('rows');
        const query = renderQuery(fetcher, { enabled: false, deps: [] }, { visible: true });
        await settle();

        expect(fetcher).not.toHaveBeenCalled();
        expect(query.states).toEqual(['populated']);
    });

    it('refetches on demand', async () => {
        const fetcher = vi.fn().mockResolvedValue('rows');
        const query = renderQuery(fetcher, { deps: [] }, { visible: true });
        await settle();
        expect(fetcher).toHaveBeenCalledTimes(1);

        await act(async () => {
            await query.latest().refetch();
        });
        expect(fetcher).toHaveBeenCalledTimes(2);
    });
});
