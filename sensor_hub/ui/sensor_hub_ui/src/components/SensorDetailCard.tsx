import { useCallback } from 'react';
import type { Sensor, MeasurementTypeInfo } from '../gen/aliases';
import { apiClient } from '../gen/client';
import { useCurrentReadings } from '../hooks/useCurrentReadings';
import { useScheduledQuery } from '../hooks/useScheduledQuery';
import { WidgetSwap, SensorDetailTilesLoader } from '../ui/loaders';
import Card from '../ui/Card';
import StatGrid from '../ui/StatGrid';

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
                    <StatGrid
                        stats={measurementTypes.map((mt) => {
                            const reading = sensorReadings[mt.name];
                            return {
                                key: mt.name,
                                label: mt.display_name,
                                value: reading?.numeric_value != null
                                    ? `${reading.numeric_value.toFixed(1)} ${reading.unit ?? mt.unit}`
                                    : reading?.text_state ?? '—',
                            };
                        })}
                    />
                </Card>
            )}
        </WidgetSwap>
    );
}
