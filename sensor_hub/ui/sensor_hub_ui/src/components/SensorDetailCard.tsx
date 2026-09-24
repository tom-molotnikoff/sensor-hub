import { useCallback } from 'react';
import { Paper, Typography, Grid } from '@mui/material';
import type { Sensor, MeasurementTypeInfo } from '../gen/aliases';
import { apiClient } from '../gen/client';
import { useCurrentReadings } from '../hooks/useCurrentReadings';
import { useScheduledQuery } from '../hooks/useScheduledQuery';
import { WidgetSwap, SensorDetailTilesLoader } from '../ui/loaders';
import Card from '../ui/Card';

interface SensorDetailCardProps {
    sensor: Sensor;
    onDataUpdate?: (date: Date) => void;
}

const NO_TYPES: MeasurementTypeInfo[] = [];

export default function SensorDetailCard({ sensor, onDataUpdate }: SensorDetailCardProps) {
    const readings = useCurrentReadings({ onDataUpdate });
    const sensorId = sensor.id;

    const fetcher = useCallback(async (signal: AbortSignal) => {
        const { data } = await apiClient.GET('/sensors/by-id/{id}/measurement-types', {
            params: { path: { id: sensorId } },
            signal,
        });
        return data ?? NO_TYPES;
    }, [sensorId]);

    const { data, isLoading } = useScheduledQuery(fetcher, { deps: [sensorId] });
    const measurementTypes = data ?? NO_TYPES;

    const sensorReadings = readings[sensor.name] ?? {};

    return (
        <WidgetSwap loading={isLoading} loader={<SensorDetailTilesLoader />}>
            {measurementTypes.length === 0 ? null : (
            <Card title={`${sensor.name}: Details`}>
            <Grid container spacing={1}>
                {measurementTypes.map((mt) => {
                    const reading = sensorReadings[mt.name];
                    return (
                        <Grid key={mt.name} size={{ xs: 6, sm: 4, md: 3 }}>
                            <Paper variant="outlined" sx={{ p: 1.5, textAlign: 'center' }}>
                                <Typography variant="caption" sx={{
                                    color: "text.secondary"
                                }}>
                                    {mt.display_name}
                                </Typography>
                                <Typography variant="h6" sx={{ fontWeight: 'bold' }}>
                                    {reading?.numeric_value != null
                                        ? `${reading.numeric_value.toFixed(1)} ${reading.unit ?? mt.unit}`
                                        : reading?.text_state ?? '—'}
                                </Typography>
                            </Paper>
                        </Grid>
                    );
                })}
            </Grid>
        </Card>
            )}
        </WidgetSwap>
    );
}
