import { useEffect } from 'react';
import type { WidgetProps } from '../types';
import NotificationsCard from '../../components/NotificationsCard';
import { useNotifications } from '../../providers/NotificationContext';
import { useReportWidgetUpdate } from '../WidgetUpdateContext';
import { useWidgetStateReport } from '../WidgetContext';

export default function NotificationsFeedWidget(_props: WidgetProps) {
    const reportUpdate = useReportWidgetUpdate();
    useWidgetStateReport(useNotifications().loading ? 'loading' : 'populated');
    useEffect(() => { reportUpdate(new Date()); }, [reportUpdate]);
    return <NotificationsCard showTitle={false} />;
}
