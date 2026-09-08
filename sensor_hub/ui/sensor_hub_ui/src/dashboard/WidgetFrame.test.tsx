import { act, render, screen } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import type { DashboardWidget } from '../gen/aliases';
import { registerWidget } from './WidgetRegistry';
import { useWidgetStateReport, useWidgetViewport, type WidgetState } from './WidgetContext';
import WidgetFrame from './WidgetFrame';

vi.mock('./useWidgetSubtitle', () => ({ useWidgetSubtitle: () => undefined }));

let reported: WidgetState = 'held';

function Probe() {
    useWidgetStateReport(reported);
    const { visible } = useWidgetViewport();
    return <span data-testid="probe" data-visible={String(visible)}>probe</span>;
}

function Exploding(): never {
    throw new Error('widget blew up');
}

registerWidget({
    type: 'test-probe',
    label: 'Probe',
    description: 'Reports whatever the test asks it to',
    kind: 'informational',
    component: Probe,
    defaultConfig: {},
    defaultLayout: { w: 1, h: 1 },
});

registerWidget({
    type: 'test-exploding',
    label: 'Exploding',
    description: 'Throws on render',
    kind: 'informational',
    component: Exploding,
    defaultConfig: {},
    defaultLayout: { w: 1, h: 1 },
});

function widgetOf(type: string): DashboardWidget {
    return { id: 'w1', type, config: {}, layout: { x: 0, y: 0, w: 1, h: 1 } };
}

function renderFrame(type = 'test-probe', isEditing = false) {
    return render(
        <WidgetFrame widget={widgetOf(type)} isEditing={isEditing} onRemove={() => {}} onConfigure={() => {}} />,
    );
}

function probeVisible(): string | undefined {
    return screen.getByTestId('probe').dataset.visible;
}

function frameState(): string | null {
    return document.querySelector('[data-widget-state]')?.getAttribute('data-widget-state') ?? null;
}

describe('WidgetFrame', () => {
    beforeEach(() => {
        reported = 'held';
    });

    afterEach(() => {
        Reflect.deleteProperty(globalThis, 'IntersectionObserver');
    });

    it.each<WidgetState>(['held', 'loading', 'populated', 'error'])(
        'exposes data-widget-state=%s as the widget reports it',
        (state) => {
            reported = state;
            renderFrame();
            expect(frameState()).toBe(state);
        },
    );

    it('reports error for an unknown widget type', () => {
        render(
            <WidgetFrame widget={widgetOf('not-registered')} isEditing={false} onRemove={() => {}} onConfigure={() => {}} />,
        );
        expect(frameState()).toBe('error');
    });

    it('reports error when the widget throws', () => {
        const consoleError = vi.spyOn(console, 'error').mockImplementation(() => {});
        renderFrame('test-exploding');
        expect(frameState()).toBe('error');
        consoleError.mockRestore();
    });

    it('reports populated while the dashboard is being edited', () => {
        reported = 'held';
        renderFrame('test-probe', true);
        expect(frameState()).toBe('populated');
    });

    it('reports the frame visible when IntersectionObserver is unavailable', () => {
        renderFrame();
        expect(probeVisible()).toBe('true');
    });

    it('holds the frame until the observer says it intersects, and observes the viewport plus a third below it', () => {
        let notify: (entries: { isIntersecting: boolean }[]) => void = () => {};
        let seenOptions: IntersectionObserverInit | undefined;
        const observe = vi.fn();

        class FakeIntersectionObserver {
            constructor(callback: (entries: { isIntersecting: boolean }[]) => void, options?: IntersectionObserverInit) {
                notify = callback;
                seenOptions = options;
            }
            observe = observe;
            disconnect = vi.fn();
            unobserve = vi.fn();
        }
        Object.defineProperty(globalThis, 'IntersectionObserver', {
            configurable: true,
            writable: true,
            value: FakeIntersectionObserver,
        });

        renderFrame();

        expect(observe).toHaveBeenCalledTimes(1);
        expect(seenOptions?.rootMargin).toBe('0px 0px 33% 0px');
        expect(probeVisible()).toBe('false');

        act(() => notify([{ isIntersecting: true }]));
        expect(probeVisible()).toBe('true');
    });
});
