import type { WidgetProps } from '../types';
import { useState, useEffect, useRef, useCallback } from 'react';
import { Box, Typography } from '@mui/material';
import { useSensorContext } from '../../hooks/useSensorContext';
import { apiClient } from '../../gen/client';
import { useScheduledQuery } from '../../hooks/useScheduledQuery';
import { heatColour, useChartColours } from '../../ui/theme/chartColours';
import { parseUTCTime } from '../../tools/Utils';
import NeedsConfiguration from '../NeedsConfiguration';
import { useReportWidgetUpdate } from '../WidgetUpdateContext';
import { WidgetSwap, RippleHeatmapLoader } from '../widget-loaders';

function valueToColor(value: number, low: number, high: number): string {
    return heatColour((value - low) / (high - low));
}

interface DayData {
    day: number;
    avg: number | null;
}

const EMPTY_DAYS: DayData[] = [];

export default function HeatmapWidget({ config }: WidgetProps) {
    const { sensors } = useSensorContext();
    const chartColours = useChartColours();
    const reportUpdate = useReportWidgetUpdate();
    const [cellSize, setCellSize] = useState(28);
    const gridRef = useRef<HTMLDivElement>(null);

    const low = typeof config.scaleMin === 'number' ? config.scaleMin : 10;
    const high = typeof config.scaleMax === 'number' ? config.scaleMax : 30;
    const measurementType = config.measurementType as string | undefined;

    const sensorId = config.sensorId as number | undefined;
    const sensor = sensorId ? sensors.find((s) => s.id === sensorId) : undefined;
    const sensorName = sensor?.name;

    const fetcher = useCallback(async (signal: AbortSignal): Promise<DayData[]> => {
        const now = new Date();
        const start = new Date(now.getTime() - 30 * 24 * 60 * 60 * 1000);

        const { data } = await apiClient.GET('/readings/between', {
            params: {
                query: {
                    start: start.toISOString().slice(0, 10),
                    end: now.toISOString().slice(0, 10),
                    type: measurementType,
                    sensor: sensorName,
                },
            },
            signal,
        });

        const grouped: Record<string, number[]> = {};
        for (const r of data?.readings ?? []) {
            const dateKey = parseUTCTime(r.time).toISOString().slice(0, 10);
            if (!grouped[dateKey]) grouped[dateKey] = [];
            grouped[dateKey].push(r.numeric_value ?? 0);
        }

        const result: DayData[] = [];
        for (let i = 29; i >= 0; i--) {
            const d = new Date(now.getTime() - i * 24 * 60 * 60 * 1000);
            const values = grouped[d.toISOString().slice(0, 10)];
            result.push({
                day: d.getDate(),
                avg: values ? values.reduce((s, t) => s + t, 0) / values.length : null,
            });
        }
        return result;
    }, [measurementType, sensorName]);

    const { data, isLoading } = useScheduledQuery(fetcher, {
        enabled: !!sensorName && !!measurementType,
        deps: [sensorName, measurementType],
    });
    const days = data ?? EMPTY_DAYS;

    useEffect(() => {
        if (data) reportUpdate(new Date());
    }, [data, reportUpdate]);

    const cols = 7;
    const rows = Math.ceil(days.length / cols) || 1;
    const gap = 4;

    const recalc = useCallback(() => {
        const el = gridRef.current;
        if (!el) return;
        const { width, height } = el.getBoundingClientRect();
        const maxFromW = (width - (cols - 1) * gap) / cols;
        const maxFromH = (height - (rows - 1) * gap) / rows;
        setCellSize(Math.max(12, Math.floor(Math.min(maxFromW, maxFromH))));
    }, [rows]);

    useEffect(() => {
        const el = gridRef.current;
        if (!el) return;
        const observer = new ResizeObserver(recalc);
        observer.observe(el);
        return () => observer.disconnect();
    }, [recalc]);

    if (!sensor || !measurementType) {
        return <NeedsConfiguration message="Select a sensor and measurement type" />;
    }

    const firstMonth = days.length > 0 ? new Date(new Date().getTime() - 29 * 24 * 60 * 60 * 1000).toLocaleString('default', { month: 'long' }) : '';
    const lastMonth = new Date().toLocaleString('default', { month: 'long' });
    const monthLabel = firstMonth === lastMonth ? firstMonth : `${firstMonth} → ${lastMonth}`;

    return (
        <Box sx={{ display: 'flex', flexDirection: 'column', height: '100%', p: 1, overflow: 'hidden' }}>
            <Typography
                variant="caption"
                sx={{
                    color: "text.secondary",
                    mb: 0.5,
                    textAlign: 'center',
                    flexShrink: 0
                }}>{monthLabel}</Typography>
            <Box
                ref={gridRef}
                sx={{
                    flex: 1,
                    minHeight: 0,
                    display: 'flex',
                    alignItems: 'center',
                    justifyContent: 'center',
                }}
            >
                <WidgetSwap loading={isLoading} loader={<RippleHeatmapLoader columns={cols} count={30} />}>
                <Box
                    sx={{
                        display: 'grid',
                        gridTemplateColumns: `repeat(${cols}, ${cellSize}px)`,
                        gap: `${gap}px`,
                    }}
                >
                    {days.map((d, i) => (
                        <Box
                            key={i}
                            sx={{
                                width: cellSize,
                                height: cellSize,
                                borderRadius: 0.5,
                                display: 'flex',
                                alignItems: 'center',
                                justifyContent: 'center',
                                backgroundColor: d.avg !== null ? valueToColor(d.avg, low, high) : chartColours.noData,
                                color: d.avg !== null ? 'common.white' : chartColours.axisText,
                                fontSize: Math.max(9, cellSize * 0.35),
                                fontWeight: 'bold',
                            }}
                        >
                            {d.day}
                        </Box>
                    ))}
                </Box>
                </WidgetSwap>
            </Box>
        </Box>
    );
}
