import type { WidgetProps } from '../types';
import CurrentTemperatures from '../../components/CurrentTemperatures';
import { useCurrentReadingsReady } from '../../hooks/useCurrentReadings';
import { useReportWidgetUpdate } from '../WidgetUpdateContext';
import { useWidgetStateReport } from '../WidgetContext';

export default function LiveReadingsTableWidget(_props: WidgetProps) {
    const reportUpdate = useReportWidgetUpdate();
    useWidgetStateReport(useCurrentReadingsReady() ? 'populated' : 'loading');
    return <CurrentTemperatures cardHeight="100%" showTitle={false} onDataUpdate={reportUpdate} />;
}

