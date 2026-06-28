import { useEffect, useMemo } from 'react';
import type { WidgetProps } from '../types';
import { Box, LinearProgress, Typography } from '@mui/material';
import { useSensorContext } from '../../hooks/useSensorContext';
import useSensorHealthHistory from '../../hooks/useSensorHealthHistory';
import { useReportWidgetUpdate } from '../WidgetUpdateContext';
import { buildHealthWindowModel, formatDurationShort, formatWindowLabel } from '../../health/healthWindow';
import { useProperties } from '../../hooks/useProperties';
import { WidgetSwap, IndeterminateBarLoader } from '../widget-loaders';

export default function UptimeWidget({ config }: WidgetProps) {
    const { sensors } = useSensorContext();
    const reportUpdate = useReportWidgetUpdate();
    const properties = useProperties();
    const sensorId = config.sensorId as number | undefined;
    const sensor = sensorId ? sensors.find((s) => s.id === sensorId) : undefined;
    const sensorName = sensor?.name ?? '';

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

    const getColor = (pct: number): 'success' | 'warning' | 'error' => {
        if (pct > 90) return 'success';
        if (pct >= 70) return 'warning';
        return 'error';
    };

    if (!sensor) {
        return (
            <Box sx={{ display: 'flex', alignItems: 'center', justifyContent: 'center', height: '100%' }}>
                <Typography sx={{
                    color: "text.secondary"
                }}>Configure sensor</Typography>
            </Box>
        );
    }

    return (
        <WidgetSwap loading={historyLoading && !model} loader={<IndeterminateBarLoader />}>
        <Box sx={{ display: 'flex', flexDirection: 'column', alignItems: 'center', justifyContent: 'center', height: '100%', p: 2, gap: 2 }}>
            <Typography variant="h3" sx={{ fontWeight: 'bold' }}>
                {model ? `${uptime.toFixed(1)}%` : '—'}
            </Typography>
            <Box sx={{ width: '80%' }}>
                <LinearProgress
                    variant="determinate"
                    value={uptime}
                    color={getColor(uptime)}
                    sx={{ height: 10, borderRadius: 5 }}
                />
            </Box>
            {model && (
                <>
                    <Typography variant="body2" align="center" sx={{
                        color: "text.secondary"
                    }}>
                        Good for {formatDurationShort(model.durationsMs.good)} of last {formatWindowLabel(model.windowDurationMs)}
                    </Typography>
                    <Typography variant="caption" align="center" sx={{
                        color: "text.secondary"
                    }}>
                        Bad {formatDurationShort(model.durationsMs.bad)} · Unknown {formatDurationShort(model.durationsMs.unknown)}
                    </Typography>
                </>
            )}
            <Typography variant="subtitle2" sx={{
                color: "text.secondary"
            }}>{sensor.name}</Typography>
        </Box>
        </WidgetSwap>
    );
}
