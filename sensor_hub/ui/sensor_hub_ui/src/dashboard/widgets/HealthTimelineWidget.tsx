import { useEffect } from 'react';
import type { WidgetProps } from '../types';
import SensorHealthHistoryChart from '../../components/SensorHealthHistoryChart';
import { useSensorContext } from '../../hooks/useSensorContext';
import NeedsConfiguration from '../NeedsConfiguration';
import { useReportWidgetUpdate } from '../WidgetUpdateContext';

export default function HealthTimelineWidget({ config }: WidgetProps) {
    const { sensors } = useSensorContext();
    const reportUpdate = useReportWidgetUpdate();
    const sensorId = config.sensorId as number | undefined;
    const sensor = sensorId ? sensors.find((s) => s.id === sensorId) : sensors[0];

    useEffect(() => { reportUpdate(new Date()); }, [reportUpdate]);

    if (!sensor) {
        return <NeedsConfiguration message="Select a sensor in widget settings" />;
    }

    return <SensorHealthHistoryChart sensor={sensor} />;
}
