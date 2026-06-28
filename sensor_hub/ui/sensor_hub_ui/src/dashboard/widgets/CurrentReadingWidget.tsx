import type { WidgetProps } from '../types';
import { Box, Typography } from '@mui/material';
import { useSensorContext } from '../../hooks/useSensorContext';
import { useCurrentReadings, useCurrentReadingsReady } from '../../hooks/useCurrentReadings';
import { parseUTCTime } from '../../tools/Utils';
import NeedsConfiguration from '../NeedsConfiguration';
import { useReportWidgetUpdate } from '../WidgetUpdateContext';
import { WidgetSwap, ValuePlaceholderLoader } from '../widget-loaders';

export default function CurrentReadingWidget({ config }: WidgetProps) {
    const { sensors } = useSensorContext();
    const reportUpdate = useReportWidgetUpdate();
    const readings = useCurrentReadings({ onDataUpdate: reportUpdate });
    const ready = useCurrentReadingsReady();

    const sensorId = config.sensorId as number | undefined;
    const measurementType = config.measurementType as string | undefined;
    const sensor = sensorId ? sensors.find((s) => s.id === sensorId) : undefined;

    if (!sensor || !measurementType) {
        return <NeedsConfiguration message="Select a sensor and measurement type" />;
    }

    const reading = readings[sensor.name]?.[measurementType];
    // Loading only while we have no reading for this widget AND no snapshot has
    // arrived yet. Once the snapshot is in, a missing reading is genuinely empty.
    const isLoading = !reading && !ready;

    return (
        <WidgetSwap loading={isLoading} loader={<ValuePlaceholderLoader />}>
            <Box sx={{ display: 'flex', flexDirection: 'column', alignItems: 'center', justifyContent: 'center', height: '100%', p: 2 }}>
                <Typography variant="subtitle1" sx={{
                    color: "text.secondary"
                }}>{sensor.name}</Typography>
                <Typography variant="h1" sx={{ fontSize: '4rem', fontWeight: 'bold', textAlign: 'center' }}>
                    {reading
                        ? reading.numeric_value != null
                            ? `${reading.numeric_value.toFixed(1)}${reading.unit ? ` ${reading.unit}` : ''}`
                            : reading.text_state ?? '—'
                        : '—'}
                </Typography>
                {reading && (
                    <Typography variant="caption" sx={{
                        color: "text.secondary"
                    }}>
                        {parseUTCTime(reading.time).toLocaleString()}
                    </Typography>
                )}
            </Box>
        </WidgetSwap>
    );
}
