import type { WidgetProps } from '../types';
import { useSensorContext } from '../../hooks/useSensorContext';
import { useCurrentReadings, useCurrentReadingsReady } from '../../hooks/useCurrentReadings';
import NeedsConfiguration from '../NeedsConfiguration';
import { useReportWidgetUpdate } from '../WidgetUpdateContext';
import { useWidgetStateReport } from '../WidgetContext';
import { WidgetSwap, ValuePlaceholderLoader } from '../../ui/loaders';
import Metric from '../../ui/Metric';

export default function CurrentReadingWidget({ config }: WidgetProps) {
    const { sensors } = useSensorContext();
    const reportUpdate = useReportWidgetUpdate();
    const readings = useCurrentReadings({ onDataUpdate: reportUpdate });
    const ready = useCurrentReadingsReady();
    useWidgetStateReport(ready ? 'populated' : 'loading');

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
            <Metric
                value={reading ? reading.numeric_value ?? reading.text_state ?? null : null}
                unit={reading?.numeric_value != null ? reading.unit || undefined : undefined}
                label={sensor.name}
                caption={reading ? new Date(reading.time).toLocaleString() : undefined}
            />
        </WidgetSwap>
    );
}
