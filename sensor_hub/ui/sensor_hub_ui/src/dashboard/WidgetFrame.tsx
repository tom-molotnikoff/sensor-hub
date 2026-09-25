import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { IconButton } from '@mui/material';
import CloseIcon from '@mui/icons-material/Close';
import SettingsIcon from '@mui/icons-material/Settings';
import { getWidget } from './WidgetRegistry';
import { useWidgetSubtitle } from './useWidgetSubtitle';
import { WidgetErrorBoundary } from './WidgetErrorBoundary';
import { useWidgetLastUpdated } from './WidgetUpdateContext';
import { WidgetUpdateProvider } from './WidgetUpdateProvider';
import RelativeTime from './RelativeTime';
import EmptyState from '../ui/EmptyState';
import Frame, { FramePlaceholder } from '../ui/Frame';
import type { ReactNode } from 'react';
import {
    WidgetStateReportContext,
    WidgetViewportContext,
    type WidgetState,
    type WidgetViewport,
} from './WidgetContext';
import type { WidgetProps } from './types';
import type { DashboardWidget } from '../gen/aliases';

export const WIDGET_VISIBILITY_MARGIN = '0px 0px 33% 0px';

function useFrameVisibility(): [boolean, (element: HTMLElement | null) => void] {
    const hasObserver = typeof IntersectionObserver !== 'undefined';
    const [visible, setVisible] = useState(!hasObserver);
    const elementRef = useRef<HTMLElement | null>(null);

    useEffect(() => {
        const element = elementRef.current;
        if (!element || !hasObserver) return;
        const observer = new IntersectionObserver(
            (entries) => setVisible(entries[entries.length - 1].isIntersecting),
            { rootMargin: WIDGET_VISIBILITY_MARGIN },
        );
        observer.observe(element);
        return () => observer.disconnect();
    }, [hasObserver]);

    const observe = useCallback((element: HTMLElement | null) => {
        elementRef.current = element;
    }, []);

    return [visible, observe];
}

interface WidgetFrameProvidersProps {
    viewport: WidgetViewport;
    reportState: (state: WidgetState) => void;
    children: ReactNode;
}

function WidgetFrameProviders({ viewport, reportState, children }: WidgetFrameProvidersProps) {
    return (
        <WidgetUpdateProvider>
            <WidgetViewportContext.Provider value={viewport}>
                <WidgetStateReportContext.Provider value={reportState}>
                    {children}
                </WidgetStateReportContext.Provider>
            </WidgetViewportContext.Provider>
        </WidgetUpdateProvider>
    );
}

interface WidgetFrameProps {
    widget: DashboardWidget;
    isEditing: boolean;
    draggable: boolean;
    onRemove: (id: string) => void;
    onConfigure: (id: string) => void;
}

function WidgetLastUpdatedBadge() {
    const lastUpdated = useWidgetLastUpdated();
    if (!lastUpdated) return null;
    return <RelativeTime date={lastUpdated} />;
}

export default function WidgetFrame({ widget, isEditing, draggable, onRemove, onConfigure }: WidgetFrameProps) {
    const definition = getWidget(widget.type);
    const subtitle = useWidgetSubtitle(definition?.type ?? widget.type, widget.config);
    const [visible, observeFrame] = useFrameVisibility();
    const [reportedState, setReportedState] = useState<WidgetState>('held');
    const reportState = useCallback((state: WidgetState) => setReportedState(state), []);
    const kind = definition?.kind ?? 'informational';
    const viewport = useMemo<WidgetViewport>(() => ({ visible, kind }), [visible, kind]);

    if (!definition) {
        return (
            <Frame
                state="error"
                actions={isEditing && (
                    <IconButton size="small" aria-label="Remove widget" onClick={() => onRemove(widget.id)}>
                        <CloseIcon fontSize="small" />
                    </IconButton>
                )}
            >
                <EmptyState title={`Unknown widget: ${widget.type}`} />
            </Frame>
        );
    }

    const Component = definition.component;
    const hasConfig = definition.configFields && definition.configFields.length > 0;
    const titleText = subtitle ? `${definition.label}: ${subtitle}` : definition.label;
    const widgetProps: WidgetProps = {
        id: widget.id,
        config: widget.config,
        isEditing,
    };

    return (
        <WidgetFrameProviders viewport={viewport} reportState={reportState}>
            <Frame
                ref={observeFrame}
                state={isEditing ? 'populated' : reportedState}
                title={titleText}
                editing={isEditing}
                dragHandle={draggable}
                actions={isEditing ? (
                    <>
                        {hasConfig && (
                            <IconButton size="small" aria-label="Configure widget" onClick={() => onConfigure(widget.id)}>
                                <SettingsIcon fontSize="small" />
                            </IconButton>
                        )}
                        <IconButton size="small" aria-label="Remove widget" onClick={() => onRemove(widget.id)}>
                            <CloseIcon fontSize="small" />
                        </IconButton>
                    </>
                ) : (
                    <WidgetLastUpdatedBadge />
                )}
            >
                {isEditing ? (
                    <FramePlaceholder label={definition.label} />
                ) : (
                    <WidgetErrorBoundary widgetId={widget.id} onRemove={onRemove} onConfigure={hasConfig ? () => onConfigure(widget.id) : undefined}>
                        <Component {...widgetProps} />
                    </WidgetErrorBoundary>
                )}
            </Frame>
        </WidgetFrameProviders>
    );
}
