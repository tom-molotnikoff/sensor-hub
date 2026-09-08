import { createContext, useContext, useEffect } from 'react';

export type WidgetKind = 'controllable' | 'informational';

export type WidgetState = 'held' | 'loading' | 'populated' | 'error';

export interface WidgetViewport {
    visible: boolean;
    kind: WidgetKind;
}

const OUTSIDE_A_FRAME: WidgetViewport = { visible: true, kind: 'informational' };

export const WidgetViewportContext = createContext<WidgetViewport>(OUTSIDE_A_FRAME);
export const WidgetStateReportContext = createContext<(state: WidgetState) => void>(() => {});

export function useWidgetViewport(): WidgetViewport {
    return useContext(WidgetViewportContext);
}

export function useReportWidgetState(): (state: WidgetState) => void {
    return useContext(WidgetStateReportContext);
}

export function useWidgetStateReport(state: WidgetState | null): void {
    const report = useReportWidgetState();
    useEffect(() => {
        if (state !== null) report(state);
    }, [report, state]);
}
