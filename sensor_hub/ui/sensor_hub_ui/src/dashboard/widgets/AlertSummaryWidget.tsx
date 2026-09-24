import type { WidgetProps } from '../types';
import type { AlertRule } from '../../gen/aliases';
import { useCallback, useEffect } from 'react';
import { List, ListItem, ListItemText, Chip } from '@mui/material';
import { apiClient } from '../../gen/client';
import { useScheduledQuery } from '../../hooks/useScheduledQuery';
import { useReportWidgetUpdate } from '../WidgetUpdateContext';
import { WidgetSwap, CascadeRowsLoader } from '../../ui/loaders';
import Card from '../../ui/Card';
import EmptyState from '../../ui/EmptyState';

const NO_RULES: AlertRule[] = [];

export default function AlertSummaryWidget(_props: WidgetProps) {
    const reportUpdate = useReportWidgetUpdate();

    const fetcher = useCallback(async (signal: AbortSignal) => {
        const { data } = await apiClient.GET('/alerts', { signal });
        return (data as AlertRule[] | null) ?? NO_RULES;
    }, []);

    const { data, isLoading } = useScheduledQuery(fetcher, { deps: [] });
    const rules = data ?? NO_RULES;

    useEffect(() => {
        if (data) reportUpdate(new Date());
    }, [data, reportUpdate]);

    return (
        <WidgetSwap loading={isLoading} loader={<CascadeRowsLoader />}>
            {rules.length === 0 ? (
                <EmptyState size="sm" title="No alert rules configured" />
            ) : (
                <Card>
                    <List dense disablePadding>
                        {rules.map((rule) => (
                            <ListItem key={rule.ID} disableGutters>
                                <ListItemText
                                    primary={rule.SensorName}
                                    secondary={`${rule.AlertType} — threshold: ${rule.HighThreshold ?? rule.LowThreshold ?? '—'}${rule.LastAlertSentAt ? ` · last: ${new Date(rule.LastAlertSentAt).toLocaleDateString()}` : ''}`}
                                />
                                <Chip
                                    label={rule.Enabled ? 'Enabled' : 'Disabled'}
                                    size="small"
                                    color={rule.Enabled ? 'success' : 'default'}
                                />
                            </ListItem>
                        ))}
                    </List>
                </Card>
            )}
        </WidgetSwap>
    );
}
