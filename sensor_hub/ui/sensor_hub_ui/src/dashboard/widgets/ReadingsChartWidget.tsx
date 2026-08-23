import { useCallback } from 'react';
import type { WidgetProps } from '../types';
import { useSensorContext } from '../../hooks/useSensorContext';
import ReadingsChart from '../../components/ReadingsChart';
import NeedsConfiguration from '../NeedsConfiguration';
import { resolveTimeRange } from '../timeRange';
import { useReportWidgetUpdate } from '../WidgetUpdateContext';

export default function ReadingsChartWidget({ config }: WidgetProps) {
    const { sensors } = useSensorContext();
    const reportUpdate = useReportWidgetUpdate();
    const measurementType = config.measurementType as string | undefined;
    const aggregationFunction = config.aggregationFunction as string | undefined;
    const resolveRange = useCallback(() => resolveTimeRange(config), [config]);

    if (!measurementType) {
        return <NeedsConfiguration message="Select a measurement type to display" />;
    }

    const pollIntervalMs = typeof config.refreshInterval === 'number' && config.refreshInterval > 0
        ? config.refreshInterval * 1000 : undefined;

    return (
        <div style={{ display: 'flex', flexDirection: 'column', flex: 1, minHeight: 0, width: '100%' }}>
            <ReadingsChart
                sensors={sensors}
                startDate={null}
                endDate={null}
                measurementType={measurementType}
                aggregationFunction={aggregationFunction}
                pollIntervalMs={pollIntervalMs}
                resolveTimeRange={resolveRange}
                onDataUpdate={reportUpdate}
            />
        </div>
    );
}
