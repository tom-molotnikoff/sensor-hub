import { useEffect } from 'react';
import type { WidgetProps } from '../types';
import WeatherForecastCard from '../../components/WeatherForecastCard';
import { useReportWidgetUpdate } from '../WidgetUpdateContext';

export default function WeatherForecastWidget(_props: WidgetProps) {
    const reportUpdate = useReportWidgetUpdate();
    useEffect(() => { reportUpdate(new Date()); }, [reportUpdate]);
    return <WeatherForecastCard />;
}
