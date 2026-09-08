import { useEffect } from 'react';
import type { WidgetProps } from '../types';
import SensorTypeCard from '../../components/SensorTypeCard';
import { useSensorContext } from '../../hooks/useSensorContext';
import { useReportWidgetUpdate } from '../WidgetUpdateContext';
import { useWidgetStateReport } from '../WidgetContext';

export default function SensorTypePieWidget(_props: WidgetProps) {
    const reportUpdate = useReportWidgetUpdate();
    useWidgetStateReport(useSensorContext().loaded ? 'populated' : 'loading');
    useEffect(() => { reportUpdate(new Date()); }, [reportUpdate]);
    return <SensorTypeCard showTitle={false} />;
}
