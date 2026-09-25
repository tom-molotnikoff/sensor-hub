import { act, render, screen } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import type { DashboardWidget } from '../gen/aliases';
import { registerAlias, registerWidget } from './WidgetRegistry';
import { useWidgetStateReport, useWidgetViewport, type WidgetState } from './WidgetContext';
import WidgetFrame from './WidgetFrame';

const subtitleTypes: string[] = [];
vi.mock('./useWidgetSubtitle', () => ({
    useWidgetSubtitle: (type: string) => {
        subtitleTypes.push(type);
        return undefined;
    },
}));

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
    compactHeight: 'content',
});

registerWidget({
    type: 'test-exploding',
    label: 'Exploding',
    description: 'Throws on render',
    kind: 'informational',
    component: Exploding,
    defaultConfig: {},
    defaultLayout: { w: 1, h: 1 },
    compactHeight: 'content',
});

registerAlias('test-probe-legacy', 'test-probe');

function widgetOf(type: string): DashboardWidget {
    return { id: 'w1', type, config: {}, layout: { x: 0, y: 0, w: 1, h: 1 } };
}

function renderFrame(type = 'test-probe', isEditing = false) {
    return render(
        <WidgetFrame widget={widgetOf(type)} isEditing={isEditing} draggable onRemove={() => {}} onConfigure={() => {}} />,
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
        subtitleTypes.splice(0, subtitleTypes.length);
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

    it('builds the subtitle from the registered type when the widget uses an alias', () => {
        renderFrame('test-probe-legacy');
        expect(new Set(subtitleTypes)).toEqual(new Set(['test-probe']));
    });

    it('reports error for an unknown widget type', () => {
        render(
            <WidgetFrame widget={widgetOf('not-registered')} isEditing={false} draggable onRemove={() => {}} onConfigure={() => {}} />,
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

    it('covers the widget with its edit placeholder, without edit actions and without remounting it', () => {
        reported = 'populated';
        const framed = (covered: boolean) => (
            <WidgetFrame widget={widgetOf('test-probe')} isEditing={false} draggable covered={covered} onRemove={() => {}} onConfigure={() => {}} />
        );
        const { rerender } = render(framed(false));
        const probe = screen.getByTestId('probe');

        rerender(framed(true));
        expect(document.querySelector('[data-ui=frame-cover] [data-ui=frame-placeholder]')).toHaveTextContent('Probe');
        expect(screen.getByTestId('probe')).toBe(probe);
        expect(frameState()).toBe('populated');
        expect(screen.queryByRole('button', { name: 'Configure widget' })).not.toBeInTheDocument();
        expect(screen.queryByRole('button', { name: 'Remove widget' })).not.toBeInTheDocument();
        expect(document.querySelector('.drag-handle')).toBeNull();

        rerender(framed(false));
        expect(document.querySelector('[data-ui=frame-placeholder]')).toBeNull();
        expect(screen.getByTestId('probe')).toBe(probe);
    });

    it('shows only the edit placeholder when covered while editing', () => {
        render(<WidgetFrame widget={widgetOf('test-probe')} isEditing draggable covered onRemove={() => {}} onConfigure={() => {}} />);

        expect(document.querySelector('[data-ui=frame-cover]')).toBeNull();
        expect(document.querySelectorAll('[data-ui=frame-placeholder]')).toHaveLength(1);
        expect(screen.getByRole('button', { name: 'Remove widget' })).toBeInTheDocument();
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
