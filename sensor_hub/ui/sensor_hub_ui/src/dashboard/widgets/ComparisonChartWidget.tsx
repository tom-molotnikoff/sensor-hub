import { useCallback } from 'react';
import type { WidgetProps } from '../types';
import { Typography } from '@mui/material';
import { useSensorContext } from '../../hooks/useSensorContext';
import { useMeasurementTypes } from '../../hooks/useMeasurementTypes';
import { useReadingsData } from '../../hooks/useReadingsData';
import {
    LineChart,
    Line,
    XAxis,
    YAxis,
    CartesianGrid,
    Tooltip,
    Legend,
    ResponsiveContainer,
} from 'recharts';
import { useChartColours } from '../../theme/chartColours';
import NeedsConfiguration from '../NeedsConfiguration';
import { resolveTimeRange } from '../timeRange';
import { useReportWidgetUpdate } from '../WidgetUpdateContext';
import { WidgetSwap, SignalTraceLoader } from '../widget-loaders';

export default function ComparisonChartWidget({ config }: WidgetProps) {
    const { sensors } = useSensorContext();
    const chartColours = useChartColours();
    const reportUpdate = useReportWidgetUpdate();
    const measurementType = config.measurementType as string | undefined;
    const aggregationFunction = config.aggregationFunction as string | undefined;
    const { measurementTypes } = useMeasurementTypes();

    const mtInfo = measurementTypes.find(mt => mt.name === measurementType);
    const yAxisLabel = measurementType
        ? {
            value: mtInfo
                ? `${mtInfo.display_name}${mtInfo.unit ? ` (${mtInfo.unit})` : ''}`
                : measurementType.charAt(0).toUpperCase() + measurementType.slice(1),
            angle: -90,
            position: 'insideLeft' as const,
            style: { textAnchor: 'middle' as const, fontSize: 12 },
        }
        : undefined;

    const selectedIds = Array.isArray(config.sensorIds) ? (config.sensorIds as number[]) : [];
    const filteredSensors = selectedIds.length > 0
        ? sensors.filter((s) => selectedIds.includes(s.id))
        : sensors;

    const pollIntervalMs = typeof config.refreshInterval === 'number' && config.refreshInterval > 0
        ? config.refreshInterval * 1000 : undefined;
    const resolveRange = useCallback(() => resolveTimeRange(config), [config]);

    const { mergedData: chartData, isLoading, error } = useReadingsData({
        startDate: null,
        endDate: null,
        sensors: filteredSensors,
        measurementType,
        aggregationFunction,
        pollIntervalMs,
        resolveTimeRange: resolveRange,
        onDataUpdate: reportUpdate,
    });

    if (!measurementType) {
        return <NeedsConfiguration message="Select a measurement type to compare" />;
    }

    if (filteredSensors.length === 0) {
        return (
            <Typography
                sx={{
                    color: "text.secondary",
                    p: 2
                }}>No sensors available</Typography>
        );
    }

    const noData = chartData.length === 0;
    const loading = isLoading && noData;

    return (
        <div style={{ flex: 1, minHeight: 0, width: '100%' }}>
            <WidgetSwap loading={loading} loader={<SignalTraceLoader />}>
                {error && noData ? (
                    <Typography sx={{ color: "text.secondary", p: 2 }}>
                        Couldn't load comparison data. It will retry automatically.
                    </Typography>
                ) : noData ? (
                    <Typography sx={{ color: "text.secondary", p: 2 }}>No data for the selected range</Typography>
                ) : (
                    <ResponsiveContainer width="100%" height="100%">
                        <LineChart data={chartData}>
                            <CartesianGrid stroke={chartColours.grid} strokeDasharray="3 3" />
                            <XAxis
                                dataKey="time"
                                tickFormatter={(t: string) => new Date(t).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })}
                                minTickGap={50}
                            />
                            <YAxis label={yAxisLabel} />
                            <Tooltip />
                            <Legend />
                            {filteredSensors.map((sensor, index) => (
                                <Line
                                    key={sensor.name}
                                    type="linear"
                                    dataKey={sensor.name}
                                    stroke={chartColours.categorical[index % chartColours.categorical.length]}
                                    dot={false}
                                    connectNulls
                                />
                            ))}
                        </LineChart>
                    </ResponsiveContainer>
                )}
            </WidgetSwap>
        </div>
    );
}
