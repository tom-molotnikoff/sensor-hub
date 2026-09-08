import type { WidgetProps } from '../types';
import { useCallback, useEffect } from 'react';
import { Box, Paper, Typography } from '@mui/material';
import { useSensorContext } from '../../hooks/useSensorContext';
import { useScheduledQuery } from '../../hooks/useScheduledQuery';
import { apiClient } from '../../gen/client';
import { useChartColours } from '../../theme/chartColours';
import NeedsConfiguration from '../NeedsConfiguration';
import { resolveTimeRange } from '../timeRange';
import { useReportWidgetUpdate } from '../WidgetUpdateContext';
import { WidgetSwap, SkeletonTilesLoader } from '../widget-loaders';

interface Stats {
    min: number;
    max: number;
    avg: number;
    unit: string;
}

export default function MinMaxAvgWidget({ config }: WidgetProps) {
    const { sensors } = useSensorContext();
    const chartColours = useChartColours();
    const reportUpdate = useReportWidgetUpdate();

    const sensorId = config.sensorId as number | undefined;
    const measurementType = config.measurementType as string | undefined;
    const sensor = sensorId ? sensors.find((s) => s.id === sensorId) : undefined;
    const sensorName = sensor?.name;

    const { startDate, endDate } = resolveTimeRange(config);
    const startIso = startDate.toISODate() ?? '';
    const endIso = endDate.toISODate() ?? '';

    const fetcher = useCallback(async (signal: AbortSignal): Promise<Stats | null> => {
        const { data } = await apiClient.GET('/readings/between', {
            params: { query: { start: startIso, end: endIso, type: measurementType, sensor: sensorName } },
            signal,
        });
        const readings = data?.readings ?? [];
        if (readings.length === 0) return null;

        const values = readings.map((r) => r.numeric_value ?? 0);
        return {
            min: Math.min(...values),
            max: Math.max(...values),
            avg: values.reduce((sum, value) => sum + value, 0) / values.length,
            unit: readings[0]?.unit ?? '',
        };
    }, [startIso, endIso, measurementType, sensorName]);

    const { data: stats, isLoading } = useScheduledQuery(fetcher, {
        enabled: !!sensorName && !!measurementType,
        deps: [sensorName, startIso, endIso, measurementType],
    });

    useEffect(() => {
        if (stats) reportUpdate(new Date());
    }, [stats, reportUpdate]);

    if (!sensor || !measurementType) {
        return <NeedsConfiguration message="Select a sensor and measurement type" />;
    }

    const statItems = stats
        ? [
            { label: 'Min', value: stats.min, color: chartColours.stat[0] },
            { label: 'Avg', value: stats.avg, color: chartColours.stat[1] },
            { label: 'Max', value: stats.max, color: chartColours.stat[2] },
        ]
        : [];

    return (
        <WidgetSwap loading={isLoading} loader={<SkeletonTilesLoader />}>
            {!stats ? (
                <Box sx={{ display: 'flex', alignItems: 'center', justifyContent: 'center', height: '100%' }}>
                    <Typography sx={{
                        color: "text.secondary"
                    }}>No data available</Typography>
                </Box>
            ) : (
                <Box sx={{ display: 'flex', flexDirection: 'column', height: '100%', p: 2 }}>
                    <Typography variant="subtitle1" sx={{ mb: 1 }}>{sensor.name}</Typography>
                    <Box sx={{ display: 'flex', flexDirection: 'row', gap: 2, flex: 1, alignItems: 'center' }}>
                        {statItems.map((item) => (
                            <Paper key={item.label} sx={{ flex: 1, p: 2, textAlign: 'center' }} elevation={1}>
                                <Typography variant="caption" sx={{ color: item.color, fontWeight: 'bold' }}>
                                    {item.label}
                                </Typography>
                                <Typography variant="h5" sx={{ color: item.color }}>
                                    {item.value.toFixed(1)}{stats.unit}
                                </Typography>
                            </Paper>
                        ))}
                    </Box>
                </Box>
            )}
        </WidgetSwap>
    );
}
