import type { WidgetProps } from '../types';
import { useState, useEffect, useRef, useCallback } from 'react';
import { Box, Typography } from '@mui/material';
import { useSensorContext } from '../../hooks/useSensorContext';
import { apiClient } from '../../gen/client';
import { requestScheduler } from '../../scheduler/requestScheduler';
import { useIsDark } from '../../theme/useIsDark';
import { parseUTCTime } from '../../tools/Utils';
import NeedsConfiguration from '../NeedsConfiguration';
import { useReportWidgetUpdate } from '../WidgetUpdateContext';
import { WidgetSwap, RippleHeatmapLoader } from '../widget-loaders';

function valueToColor(value: number, low: number, high: number): string {
    const ratio = Math.max(0, Math.min(1, (value - low) / (high - low)));

    // Blue (cold) → Cyan → Green (mid) → Yellow → Red (hot)
    const stops = [
        [33, 102, 172],   // 0.00 — blue
        [44, 162, 195],   // 0.25 — cyan
        [68, 179, 96],    // 0.50 — green
        [253, 200, 47],   // 0.75 — yellow
        [215, 48, 39],    // 1.00 — red
    ];

    const idx = ratio * (stops.length - 1);
    const lo = Math.min(Math.floor(idx), stops.length - 2);
    const t = idx - lo;
    const r = Math.round(stops[lo][0] + t * (stops[lo + 1][0] - stops[lo][0]));
    const g = Math.round(stops[lo][1] + t * (stops[lo + 1][1] - stops[lo][1]));
    const b = Math.round(stops[lo][2] + t * (stops[lo + 1][2] - stops[lo][2]));
    return `rgb(${r},${g},${b})`;
}

interface DayData {
    day: number;
    avg: number | null;
}

export default function HeatmapWidget({ config }: WidgetProps) {
    const { sensors } = useSensorContext();
    const isDark = useIsDark();
    const reportUpdate = useReportWidgetUpdate();
    const [days, setDays] = useState<DayData[]>([]);
    const [loading, setLoading] = useState(true);
    const [cellSize, setCellSize] = useState(28);
    const gridRef = useRef<HTMLDivElement>(null);

    const low = typeof config.scaleMin === 'number' ? config.scaleMin : 10;
    const high = typeof config.scaleMax === 'number' ? config.scaleMax : 30;
    const measurementType = config.measurementType as string | undefined;
    const noDataColor = isDark ? '#333333' : '#E0D8D0';
    const noDataTextColor = isDark ? '#A0A0A0' : '#5C5C5C';

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

    const sensorId = config.sensorId as number | undefined;
    const sensor = sensorId ? sensors.find((s) => s.id === sensorId) : undefined;

    useEffect(() => {
        if (!sensor) return;

        setLoading(true);
        const now = new Date();
        const start = new Date(now.getTime() - 30 * 24 * 60 * 60 * 1000);

        requestScheduler.schedule('normal', () => apiClient.GET('/readings/between', { params: { query: { start: start.toISOString().slice(0, 10), end: now.toISOString().slice(0, 10), measurement_type: measurementType } } })).then(({ data: response }) => {
            const sensorReadings = (response?.readings ?? []).filter((r) => r.sensor_name === sensor.name);
            const grouped: Record<string, number[]> = {};

            for (const r of sensorReadings) {
                const dateKey = parseUTCTime(r.time).toISOString().slice(0, 10);
                if (!grouped[dateKey]) grouped[dateKey] = [];
                grouped[dateKey].push(r.numeric_value ?? 0);
            }

            const result: DayData[] = [];
            for (let i = 29; i >= 0; i--) {
                const d = new Date(now.getTime() - i * 24 * 60 * 60 * 1000);
                const key = d.toISOString().slice(0, 10);
                const values = grouped[key];
                result.push({
                    day: d.getDate(),
                    avg: values ? values.reduce((s, t) => s + t, 0) / values.length : null,
                });
            }
            setDays(result);
            reportUpdate(new Date());
        }).finally(() => setLoading(false));
    }, [sensor, measurementType]);

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
                <WidgetSwap loading={loading && days.length === 0} loader={<RippleHeatmapLoader columns={cols} count={30} />}>
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
                                backgroundColor: d.avg !== null ? valueToColor(d.avg, low, high) : noDataColor,
                                color: d.avg !== null ? '#fff' : noDataTextColor,
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
