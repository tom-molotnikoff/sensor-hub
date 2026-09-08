import { useEffect } from 'react';
import type { WidgetProps } from '../types';
import TotalReadingsForEachSensorCard from '../../components/TotalReadingsForEachSensorCard';
import { useReportWidgetUpdate } from '../WidgetUpdateContext';

const READING_STATS_POLL_MS = 60000;

export default function ReadingStatsWidget(_props: WidgetProps) {
    const reportUpdate = useReportWidgetUpdate();
    useEffect(() => { reportUpdate(new Date()); }, [reportUpdate]);
    return <TotalReadingsForEachSensorCard showTitle={false} pollIntervalMs={READING_STATS_POLL_MS} />;
}
