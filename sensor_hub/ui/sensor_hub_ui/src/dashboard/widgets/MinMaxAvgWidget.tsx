import type { WidgetProps } from '../types';
import { useState, useEffect } from 'react';
import { Box, Paper, Typography } from '@mui/material';
import { useSensorContext } from '../../hooks/useSensorContext';
import { apiClient } from '../../gen/client';
import { requestScheduler } from '../../scheduler/requestScheduler';
import { useChartColours } from '../../theme/chartColours';
import NeedsConfiguration from '../NeedsConfiguration';
import { resolveTimeRange } from '../timeRange';
import { useReportWidgetUpdate } from '../WidgetUpdateContext';

export default function MinMaxAvgWidget({ config }: WidgetProps) {
    const { sensors } = useSensorContext();
    const chartColours = useChartColours();
    const reportUpdate = useReportWidgetUpdate();
    const [stats, setStats] = useState<{ min: number; max: number; avg: number; unit: string } | null>(null);

    const sensorId = config.sensorId as number | undefined;
    const measurementType = config.measurementType as string | undefined;
    const sensor = sensorId ? sensors.find((s) => s.id === sensorId) : undefined;

    const { startDate, endDate } = resolveTimeRange(config);
    const startIso = startDate.toISODate() ?? '';
    const endIso = endDate.toISODate() ?? '';

    useEffect(() => {
        if (!sensor) return;

        requestScheduler.schedule('normal', () => apiClient.GET('/readings/between', { params: { query: { start: startIso, end: endIso, measurement_type: measurementType } } })).then(({ data: response }) => {
            const sensorReadings = (response?.readings ?? []).filter((r) => r.sensor_name === sensor.name);
            if (sensorReadings.length === 0) {
                setStats(null);
                return;
            }
            const nums = sensorReadings.map((r) => r.numeric_value ?? 0);
            const min = Math.min(...nums);
            const max = Math.max(...nums);
            const avg = nums.reduce((sum, t) => sum + t, 0) / nums.length;
            const unit = sensorReadings[0]?.unit ?? '';
            setStats({ min, max, avg, unit });
            reportUpdate(new Date());
        });
    }, [sensor, startIso, endIso, measurementType]);

    if (!sensor || !measurementType) {
        return <NeedsConfiguration message="Select a sensor and measurement type" />;
    }

    if (!stats) {
        return (
            <Box sx={{ display: 'flex', alignItems: 'center', justifyContent: 'center', height: '100%' }}>
                <Typography sx={{
                    color: "text.secondary"
                }}>No data available</Typography>
            </Box>
        );
    }

    const statItems = [
        { label: 'Min', value: stats.min, color: chartColours.stat[0] },
        { label: 'Avg', value: stats.avg, color: chartColours.stat[1] },
        { label: 'Max', value: stats.max, color: chartColours.stat[2] },
    ];

    return (
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
    );
}
