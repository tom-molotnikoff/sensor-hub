import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { Paper, IconButton, Box, Typography, Skeleton } from '@mui/material';
import CloseIcon from '@mui/icons-material/Close';
import SettingsIcon from '@mui/icons-material/Settings';
import DragIndicatorIcon from '@mui/icons-material/DragIndicator';
import { getWidget } from './WidgetRegistry';
import { useWidgetSubtitle } from './useWidgetSubtitle';
import { WidgetErrorBoundary } from './WidgetErrorBoundary';
import { useWidgetLastUpdated } from './WidgetUpdateContext';
import { WidgetUpdateProvider } from './WidgetUpdateProvider';
import RelativeTime from './RelativeTime';
import type { ReactNode } from 'react';
import {
    WidgetStateReportContext,
    WidgetViewportContext,
    type WidgetState,
    type WidgetViewport,
} from './WidgetContext';
import type { WidgetProps } from './types';
import type { DashboardWidget } from '../gen/aliases';

function EditPlaceholder({ label }: { label: string }) {
    return (
        <Box sx={{
            height: '100%',
            display: 'flex',
            flexDirection: 'column',
            alignItems: 'center',
            justifyContent: 'center',
            gap: 1,
            p: 2,
            opacity: 0.5,
        }}>
            <Typography variant="body2" sx={{
                color: "text.secondary"
            }}>{label}</Typography>
            <Box sx={{ width: '80%', display: 'flex', flexDirection: 'column', gap: 0.5 }}>
                <Skeleton variant="rectangular" height={8} />
                <Skeleton variant="rectangular" height={8} width="60%" />
                <Skeleton variant="rectangular" height={8} width="40%" />
            </Box>
        </Box>
    );
}

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
    onRemove: (id: string) => void;
    onConfigure: (id: string) => void;
}

function WidgetLastUpdatedBadge() {
    const lastUpdated = useWidgetLastUpdated();
    if (!lastUpdated) return null;
    return <RelativeTime date={lastUpdated} />;
}

export default function WidgetFrame({ widget, isEditing, onRemove, onConfigure }: WidgetFrameProps) {
    const definition = getWidget(widget.type);
    const subtitle = useWidgetSubtitle(widget.type, widget.config);
    const [visible, observeFrame] = useFrameVisibility();
    const [reportedState, setReportedState] = useState<WidgetState>('held');
    const reportState = useCallback((state: WidgetState) => setReportedState(state), []);
    const kind = definition?.kind ?? 'informational';
    const viewport = useMemo<WidgetViewport>(() => ({ visible, kind }), [visible, kind]);

    if (!definition) {
        return (
            <Paper data-widget-state="error" sx={{ p: 2, height: '100%', display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
                <Typography color="error">Unknown widget: {widget.type}</Typography>
            </Paper>
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
            <Paper
                ref={observeFrame}
                data-widget-state={isEditing ? 'populated' : reportedState}
                elevation={isEditing ? 3 : 1}
                sx={{
                    height: '100%',
                    display: 'flex',
                    flexDirection: 'column',
                    overflow: 'hidden',
                    border: isEditing ? '1px dashed' : '1px solid',
                    borderColor: isEditing ? 'primary.main' : 'divider',
                    borderRadius: 2,
                    position: 'relative',
                    userSelect: isEditing ? 'none' : 'auto',
                }}
            >
                <Box
                    className={isEditing ? 'drag-handle' : undefined}
                    sx={{
                        display: 'flex',
                        alignItems: 'center',
                        justifyContent: 'space-between',
                        px: 1.5,
                        py: 0.5,
                        borderBottom: '1px solid',
                        borderColor: 'divider',
                        flexShrink: 0,
                        ...(isEditing && {
                            bgcolor: 'action.hover',
                            cursor: 'grab',
                        }),
                    }}
                >
                    <Box sx={{ display: 'flex', alignItems: 'center', gap: 0.5, minWidth: 0 }}>
                        {isEditing && <DragIndicatorIcon fontSize="small" color="action" />}
                        <Typography variant="caption" noWrap sx={{
                            color: "text.secondary"
                        }}>{titleText}</Typography>
                    </Box>
                    <Box sx={{ display: 'flex', alignItems: 'center', gap: 0.5, flexShrink: 0 }}>
                        {!isEditing && <WidgetLastUpdatedBadge />}
                        {isEditing && (
                            <>
                                {hasConfig && (
                                    <IconButton size="small" onClick={() => onConfigure(widget.id)}>
                                        <SettingsIcon fontSize="small" />
                                    </IconButton>
                                )}
                                <IconButton size="small" onClick={() => onRemove(widget.id)}>
                                    <CloseIcon fontSize="small" />
                                </IconButton>
                            </>
                        )}
                    </Box>
                </Box>
                <Box sx={{
                    flex: 1,
                    minHeight: 0,
                    overflow: 'hidden',
                    p: isEditing ? 1 : 0,
                    '& > *': { height: '100%', width: '100%' },
                }}>
                    {isEditing ? (
                        <EditPlaceholder label={definition.label} />
                    ) : (
                        <WidgetErrorBoundary widgetId={widget.id} onRemove={onRemove} onConfigure={hasConfig ? () => onConfigure(widget.id) : undefined}>
                            <Component {...widgetProps} />
                        </WidgetErrorBoundary>
                    )}
                </Box>
            </Paper>
        </WidgetFrameProviders>
    );
}
