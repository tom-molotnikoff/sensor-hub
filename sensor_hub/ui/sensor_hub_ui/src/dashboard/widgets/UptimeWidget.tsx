import { useEffect, useMemo } from 'react';
import type { WidgetProps } from '../types';
import TuneIcon from '@mui/icons-material/Tune';
import { useSensorContext } from '../../hooks/useSensorContext';
import useSensorHealthHistory from '../../hooks/useSensorHealthHistory';
import { useReportWidgetUpdate } from '../WidgetUpdateContext';
import { useWidgetStateReport } from '../WidgetContext';
import { buildHealthWindowModel, formatDurationShort, formatWindowLabel } from '../../health/healthWindow';
import { useProperties } from '../../hooks/useProperties';
import { WidgetSwap, IndeterminateBarLoader } from '../../ui/loaders';
import type { StatusKey } from '../../ui/theme';
import EmptyState from '../../ui/EmptyState';
import Metric from '../../ui/Metric';

export default function UptimeWidget({ config }: WidgetProps) {
    const { sensors } = useSensorContext();
    const reportUpdate = useReportWidgetUpdate();
    const properties = useProperties();
    const sensorId = config.sensorId as number | undefined;
    const sensor = sensorId ? sensors.find((s) => s.id === sensorId) : undefined;
    const sensorName = sensor?.name ?? '';
    useWidgetStateReport(sensorId ? null : 'populated');

    const [history, , historyLoading] = useSensorHealthHistory(sensorName);

    useEffect(() => {
        if (history.length > 0) reportUpdate(new Date());
    }, [history, reportUpdate]);

    const model = useMemo(() => {
        if (history.length === 0) return null;
        const now = new Date();
        const configuredRetentionDays = Number.parseInt(properties['health.history.retention.days'] ?? '', 10);
        const windowStart = Number.isFinite(configuredRetentionDays) && configuredRetentionDays > 0
            ? new Date(now.getTime() - configuredRetentionDays * 24 * 60 * 60 * 1000)
            : new Date(history.reduce((earliest, entry) => {
                return new Date(entry.recorded_at).getTime() < new Date(earliest).getTime() ? entry.recorded_at : earliest;
            }, history[0].recorded_at));
        return buildHealthWindowModel(history, {
            windowStart,
            now,
        });
    }, [history, properties]);

    const uptime = model ? Math.min(100, Math.max(0, model.goodRatio * 100)) : 0;

    const getStatus = (pct: number): StatusKey => {
        if (pct > 90) return 'ok';
        if (pct >= 70) return 'warn';
        return 'bad';
    };
    const status = getStatus(uptime);

    if (!sensor) {
        return <EmptyState size="sm" icon={<TuneIcon fontSize="large" />} title="Configure sensor" />;
    }

    return (
        <WidgetSwap loading={historyLoading && !model} loader={<IndeterminateBarLoader />}>
            <Metric
                value={model ? `${uptime.toFixed(1)}%` : null}
                tone={model ? `status.${status}.strong` : undefined}
                label={model ? `Good for ${formatDurationShort(model.durationsMs.good)} of last ${formatWindowLabel(model.windowDurationMs)}` : undefined}
                caption={model ? `Bad ${formatDurationShort(model.durationsMs.bad)} · Unknown ${formatDurationShort(model.durationsMs.unknown)}` : undefined}
            />
        </WidgetSwap>
    );
}
