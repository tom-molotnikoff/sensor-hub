import type { WidgetProps } from '../types';
import { Box, CircularProgress, Typography } from '@mui/material';
import { useSensorContext } from '../../hooks/useSensorContext';
import { useCurrentReadings, useCurrentReadingsReady } from '../../hooks/useCurrentReadings';
import NeedsConfiguration from '../NeedsConfiguration';
import { useReportWidgetUpdate } from '../WidgetUpdateContext';
import { WidgetSwap, CircularDrawLoader } from '../widget-loaders';

export default function GaugeWidget({ config }: WidgetProps) {
    const { sensors } = useSensorContext();
    const reportUpdate = useReportWidgetUpdate();
    const readings = useCurrentReadings({ onDataUpdate: reportUpdate });
    const ready = useCurrentReadingsReady();

    const sensorId = config.sensorId as number | undefined;
    const measurementType = config.measurementType as string | undefined;
    const min = (config.min as number) ?? 0;
    const max = (config.max as number) ?? 40;
    const sensor = sensorId ? sensors.find((s) => s.id === sensorId) : undefined;

    if (!sensor || !measurementType) {
        return <NeedsConfiguration message="Select a sensor and measurement type" />;
    }

    const sensorReadings = readings[sensor.name];
    const reading = sensorReadings?.[measurementType];
    const value = reading?.numeric_value ?? null;
    const unit = reading?.unit ?? '';
    const percentage = value !== null ? Math.max(0, Math.min(100, ((value - min) / (max - min)) * 100)) : 0;
    const isLoading = value === null && !ready;

    const getColor = (pct: number) => {
        if (pct < 33) return '#1976d2';
        if (pct <= 66) return '#4caf50';
        return '#d32f2f';
    };

    return (
        <WidgetSwap loading={isLoading} loader={<CircularDrawLoader />}>
        <Box sx={{ display: 'flex', flexDirection: 'column', alignItems: 'center', justifyContent: 'center', height: '100%', p: 2 }}>
            <Box sx={{ position: 'relative', display: 'inline-flex' }}>
                <CircularProgress
                    variant="determinate"
                    value={value !== null ? percentage : 0}
                    size={140}
                    thickness={6}
                    sx={{
                        transform: 'rotate(-90deg) !important',
                        color: getColor(percentage),
                    }}
                />
                <Box sx={{ position: 'absolute', top: 0, left: 0, bottom: 0, right: 0, display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
                    <Typography variant="h4" sx={{ fontWeight: 'bold' }}>
                        {value !== null ? `${value.toFixed(1)}${unit}` : '—'}
                    </Typography>
                </Box>
            </Box>
            <Typography
                variant="subtitle2"
                sx={{
                    color: "text.secondary",
                    mt: 1
                }}>
                {sensor.name}
            </Typography>
        </Box>
        </WidgetSwap>
    );
}
