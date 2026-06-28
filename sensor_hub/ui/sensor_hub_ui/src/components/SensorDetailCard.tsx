import { useEffect, useState } from 'react';
import { Paper, Typography, Grid } from '@mui/material';
import type { Sensor, MeasurementTypeInfo } from '../gen/aliases';
import { apiClient } from '../gen/client';
import { useCurrentReadings } from '../hooks/useCurrentReadings';
import LayoutCard from '../tools/LayoutCard';
import { TypographyH2 } from '../tools/Typography';
import { WidgetSwap, SensorDetailTilesLoader } from '../dashboard/widget-loaders';

interface SensorDetailCardProps {
    sensor: Sensor;
    onDataUpdate?: (date: Date) => void;
}

export default function SensorDetailCard({ sensor, onDataUpdate }: SensorDetailCardProps) {
    const [measurementTypes, setMeasurementTypes] = useState<MeasurementTypeInfo[]>([]);
    const [loading, setLoading] = useState(true);
    const readings = useCurrentReadings({ onDataUpdate });

    useEffect(() => {
        setLoading(true);
        apiClient.GET('/sensors/by-id/{id}/measurement-types', { params: { path: { id: sensor.id } } })
            .then(({ data }) => setMeasurementTypes(data ?? []))
            .finally(() => setLoading(false));
    }, [sensor.id]);

    const sensorReadings = readings[sensor.name] ?? {};

    return (
        <WidgetSwap loading={loading && measurementTypes.length === 0} loader={<SensorDetailTilesLoader />}>
            {measurementTypes.length === 0 ? null : (
            <LayoutCard variant="secondary" changes={{ height: '100%', width: '100%' }}>
            <TypographyH2>{sensor.name}: Details</TypographyH2>
            <Grid container spacing={1} sx={{ mt: 1 }}>
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
        </LayoutCard>
            )}
        </WidgetSwap>
    );
}
