import { useSensorContext } from '../hooks/useSensorContext';
import { useProperties } from '../hooks/useProperties';
import { useMeasurementTypes } from '../hooks/useMeasurementTypes';
import type { MeasurementTypeInfo, Sensor } from '../gen/aliases';

const CHART_TYPES = new Set(['readings-chart', 'comparison-chart']);

export function useWidgetSubtitle(type: string, config: Record<string, unknown>): string | null {
    const { sensors } = useSensorContext();
    const properties = useProperties();
    const isChart = CHART_TYPES.has(type);
    const measurementTypes = useMeasurementTypes(isChart);

    if (isChart) {
        return chartSubtitle(config, measurementTypes, sensorNames(config, sensors));
    }

    if (type === 'weather-forecast') {
        const name = properties["weather.location.name"];
        return typeof name === 'string' && name ? name : null;
    }

    if (type === 'sensor-toggle' && typeof config.sensorId === 'number') {
        const sensor = sensors.find(s => s.id === config.sensorId);
        const property = typeof config.property === 'string' && config.property ? config.property : null;
        if (!sensor) return null;
        return property ? `${sensor.name} · ${property}` : sensor.name;
    }

    if (typeof config.sensorId === 'number') {
        const sensor = sensors.find(s => s.id === config.sensorId);
        return sensor?.name ?? null;
    }

    return sensorNames(config, sensors);
}

function sensorNames(config: Record<string, unknown>, sensors: Sensor[]): string | null {
    if (!Array.isArray(config.sensorIds) || config.sensorIds.length === 0) return null;
    const names = (config.sensorIds as number[])
        .map(id => sensors.find(s => s.id === id)?.name)
        .filter(Boolean) as string[];
    return names.length > 3 ? `${names.length} sensors` : names.join(', ');
}

function chartSubtitle(config: Record<string, unknown>, measurementTypes: MeasurementTypeInfo[], sensors: string | null): string | null {
    const parts: string[] = [];
    if (typeof config.measurementType === 'string' && config.measurementType) {
        const info = measurementTypes.find(mt => mt.name === config.measurementType);
        parts.push(info?.display_name || config.measurementType);
    }
    if (typeof config.aggregationFunction === 'string' && config.aggregationFunction) {
        parts.push(config.aggregationFunction);
    }
    if (sensors) parts.push(sensors);
    return parts.length > 0 ? parts.join(' · ') : null;
}
