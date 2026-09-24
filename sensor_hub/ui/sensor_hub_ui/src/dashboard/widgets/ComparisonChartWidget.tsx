import { useCallback } from 'react';
import type { WidgetProps } from '../types';
import ShowChartOutlinedIcon from '@mui/icons-material/ShowChartOutlined';
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
} from 'recharts';
import { useChartColours } from '../../ui/theme/chartColours';
import { theme } from '../../ui/theme';
import ChartArea from '../../ui/ChartArea';
import EmptyState from '../../ui/EmptyState';
import NeedsConfiguration from '../NeedsConfiguration';
import { resolveTimeRange } from '../timeRange';
import { useReportWidgetUpdate } from '../WidgetUpdateContext';
import { useWidgetViewport } from '../WidgetContext';
import { WidgetSwap, SignalTraceLoader } from '../../ui/loaders';

export default function ComparisonChartWidget({ config }: WidgetProps) {
    const { sensors } = useSensorContext();
    const chartColours = useChartColours();
    const reportUpdate = useReportWidgetUpdate();
    const measurementType = config.measurementType as string | undefined;
    const aggregationFunction = config.aggregationFunction as string | undefined;
    const { visible } = useWidgetViewport();
    const measurementTypes = useMeasurementTypes(visible);

    const mtInfo = measurementTypes.find(mt => mt.name === measurementType);
    const yAxisLabel = measurementType
        ? {
            value: mtInfo
                ? `${mtInfo.display_name}${mtInfo.unit ? ` (${mtInfo.unit})` : ''}`
                : measurementType.charAt(0).toUpperCase() + measurementType.slice(1),
            angle: -90,
            position: 'insideLeft' as const,
            style: { textAnchor: 'middle' as const, fontSize: theme.typography.caption.fontSize },
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
        enabled: !!measurementType && filteredSensors.length > 0,
        resolveTimeRange: resolveRange,
        onDataUpdate: reportUpdate,
    });

    if (!measurementType) {
        return <NeedsConfiguration message="Select a measurement type to compare" />;
    }

    const icon = <ShowChartOutlinedIcon fontSize="large" />;

    if (filteredSensors.length === 0) {
        return <EmptyState icon={icon} title="No sensors available" />;
    }

    const noData = chartData.length === 0;
    const loading = isLoading && noData;
    const emptyState = error && noData ? (
        <EmptyState icon={icon} title="Couldn't load comparison data" description="It will retry automatically." />
    ) : noData ? (
        <EmptyState icon={icon} title="No data for the selected range" />
    ) : null;
    const showChart = !loading && emptyState === null;

    return (
        <ChartArea
            size="lg"
            placeholder={showChart ? undefined : (
                <WidgetSwap loading={loading} loader={<SignalTraceLoader />}>
                    {emptyState}
                </WidgetSwap>
            )}
        >
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
        </ChartArea>
    );
}
