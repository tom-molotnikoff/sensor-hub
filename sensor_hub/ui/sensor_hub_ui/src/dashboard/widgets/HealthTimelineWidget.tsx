import { useEffect } from 'react';
import type { WidgetProps } from '../types';
import SensorHealthHistoryChart from '../../components/SensorHealthHistoryChart';
import { useSensorContext } from '../../hooks/useSensorContext';
import { Typography } from '@mui/material';
import { useReportWidgetUpdate } from '../WidgetUpdateContext';
import { useWidgetStateReport } from '../WidgetContext';

export default function HealthTimelineWidget({ config }: WidgetProps) {
    const { sensors } = useSensorContext();
    const reportUpdate = useReportWidgetUpdate();
    const sensorId = config.sensorId as number | undefined;
    const sensor = sensorId ? sensors.find((s) => s.id === sensorId) : sensors[0];
    useWidgetStateReport(sensor ? null : 'populated');

    useEffect(() => { reportUpdate(new Date()); }, [reportUpdate]);

    if (!sensor) {
        return (
            <Typography
                sx={{
                    color: "text.secondary",
                    p: 2
                }}>Select a sensor in widget settings</Typography>
        );
    }

    return <SensorHealthHistoryChart sensor={sensor} />;
}
