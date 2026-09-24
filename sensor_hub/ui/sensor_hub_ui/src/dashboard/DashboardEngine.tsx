import { useCallback, useMemo } from 'react';
import { Alert, Button } from '@mui/material';
import AddIcon from '@mui/icons-material/Add';
import { GridLayout, useContainerWidth, type Layout, type LayoutItem } from 'react-grid-layout';
import 'react-grid-layout/css/styles.css';
import 'react-resizable/css/styles.css';
import WidgetFrame from './WidgetFrame';
import { getWidget } from './WidgetRegistry';
import type { DashboardConfig, DashboardWidget } from '../gen/aliases';
import { GRID_COLUMNS, GRID_ROW_HEIGHT } from './constants';
import DashboardSlot from '../ui/DashboardSlot';
import Stack from '../ui/Stack';
import { useTier } from '../ui/tiers';

interface DashboardEngineProps {
    config: DashboardConfig;
    isEditing: boolean;
    onLayoutChange: (widgets: DashboardWidget[]) => void;
    onRemoveWidget: (id: string) => void;
    onConfigureWidget: (id: string) => void;
    onAddWidget: () => void;
}

export default function DashboardEngine(props: DashboardEngineProps) {
    return useTier() === 'compact' ? <CompactDashboard {...props} /> : <WideDashboard {...props} />;
}

function readingOrder(a: DashboardWidget, b: DashboardWidget) {
    return a.layout.y - b.layout.y || a.layout.x - b.layout.x;
}

function CompactDashboard({ config, isEditing, onRemoveWidget, onConfigureWidget, onAddWidget }: DashboardEngineProps) {
    const widgets = useMemo(() => [...config.widgets].sort(readingOrder), [config.widgets]);

    return (
        <Stack>
            {isEditing && (
                <Alert severity="info">
                    Editing on a phone: add, configure and remove widgets. Arrange the layout on a wider screen.
                </Alert>
            )}
            {widgets.map((widget) => (
                <DashboardSlot key={widget.id} height={getWidget(widget.type)?.compactHeight ?? 'content'}>
                    <WidgetFrame
                        widget={widget}
                        isEditing={isEditing}
                        draggable={false}
                        onRemove={onRemoveWidget}
                        onConfigure={onConfigureWidget}
                    />
                </DashboardSlot>
            ))}
            {isEditing && (
                <Button variant="outlined" fullWidth startIcon={<AddIcon />} onClick={onAddWidget}>
                    Add widget
                </Button>
            )}
        </Stack>
    );
}

function WideDashboard({
    config,
    isEditing,
    onLayoutChange,
    onRemoveWidget,
    onConfigureWidget,
}: DashboardEngineProps) {
    const { width, containerRef } = useContainerWidth();

    const layout = useMemo(
        (): LayoutItem[] =>
            config.widgets.map((w) => ({
                i: w.id,
                x: w.layout.x,
                y: w.layout.y,
                w: w.layout.w,
                h: w.layout.h,
                minW: getWidget(w.type)?.minW,
                minH: getWidget(w.type)?.minH,
                maxW: getWidget(w.type)?.maxW,
                maxH: getWidget(w.type)?.maxH,
            })),
        [config.widgets],
    );

    const handleLayoutChange = useCallback(
        (changed: Layout) => {
            if (!isEditing) return;
            const updated = config.widgets.map((widget) => {
                const item = changed.find((l) => l.i === widget.id);
                if (!item) return widget;
                return {
                    ...widget,
                    layout: { x: item.x, y: item.y, w: item.w, h: item.h },
                };
            });
            onLayoutChange(updated);
        },
        [config.widgets, isEditing, onLayoutChange],
    );

    return (
        <div ref={containerRef} style={{ paddingBottom: isEditing ? 200 : 0 }}>
            <GridLayout
                width={width}
                layout={layout}
                gridConfig={{ cols: GRID_COLUMNS, rowHeight: GRID_ROW_HEIGHT, margin: [16, 16] }}
                dragConfig={{ enabled: isEditing, handle: '.drag-handle' }}
                resizeConfig={{ enabled: isEditing }}
                onLayoutChange={handleLayoutChange}
            >
                {config.widgets.map((widget) => (
                    <div key={widget.id} data-widget-id={widget.id}>
                        <WidgetFrame
                            widget={widget}
                            isEditing={isEditing}
                            draggable
                            onRemove={onRemoveWidget}
                            onConfigure={onConfigureWidget}
                        />
                    </div>
                ))}
            </GridLayout>
        </div>
    );
}
