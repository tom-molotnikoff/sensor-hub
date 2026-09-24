import type { WidgetProps } from '../types';
import { useSensorContext } from '../../hooks/useSensorContext';
import { useCurrentReadings, useCurrentReadingsReady } from '../../hooks/useCurrentReadings';
import NeedsConfiguration from '../NeedsConfiguration';
import { useReportWidgetUpdate } from '../WidgetUpdateContext';
import { useWidgetStateReport } from '../WidgetContext';
import { WidgetSwap, CircularDrawLoader } from '../../ui/loaders';
import Metric from '../../ui/Metric';

export default function GaugeWidget({ config }: WidgetProps) {
    const { sensors } = useSensorContext();
    const reportUpdate = useReportWidgetUpdate();
    const readings = useCurrentReadings({ onDataUpdate: reportUpdate });
    const ready = useCurrentReadingsReady();
    useWidgetStateReport(ready ? 'populated' : 'loading');

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
        if (pct < 33) return 'status.info.strong';
        if (pct <= 66) return 'status.ok.strong';
        return 'status.bad.strong';
    };

    return (
        <WidgetSwap loading={isLoading} loader={<CircularDrawLoader />}>
            <Metric
                value={value}
                unit={unit}
                label={sensor.name}
                dial={{ percent: value !== null ? percentage : 0, tone: getColor(percentage) }}
            />
        </WidgetSwap>
    );
}
