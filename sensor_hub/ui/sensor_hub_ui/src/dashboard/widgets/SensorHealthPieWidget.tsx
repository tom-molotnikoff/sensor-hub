import { useEffect } from 'react';
import type { WidgetProps } from '../types';
import SensorHealthCard from '../../components/SensorHealthCard';
import { useSensorContext } from '../../hooks/useSensorContext';
import { useReportWidgetUpdate } from '../WidgetUpdateContext';
import { useWidgetStateReport } from '../WidgetContext';

export default function SensorHealthPieWidget(_props: WidgetProps) {
    const reportUpdate = useReportWidgetUpdate();
    useWidgetStateReport(useSensorContext().loaded ? 'populated' : 'loading');
    useEffect(() => { reportUpdate(new Date()); }, [reportUpdate]);
    return <SensorHealthCard showTitle={false} />;
}
