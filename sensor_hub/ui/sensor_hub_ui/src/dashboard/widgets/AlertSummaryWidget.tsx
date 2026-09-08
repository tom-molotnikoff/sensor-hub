import type { WidgetProps } from '../types';
import type { AlertRule } from '../../gen/aliases';
import { useCallback, useEffect } from 'react';
import { Box, Typography, List, ListItem, ListItemText, Chip } from '@mui/material';
import { apiClient } from '../../gen/client';
import { useScheduledQuery } from '../../hooks/useScheduledQuery';
import { useReportWidgetUpdate } from '../WidgetUpdateContext';
import { WidgetSwap, CascadeRowsLoader } from '../widget-loaders';

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
                <Box sx={{ display: 'flex', alignItems: 'center', justifyContent: 'center', height: '100%' }}>
                    <Typography sx={{
                        color: "text.secondary"
                    }}>No alert rules configured</Typography>
                </Box>
            ) : (
                <Box sx={{ display: 'flex', flexDirection: 'column', height: '100%', overflow: 'hidden' }}>
                    <Box sx={{ overflow: 'auto', flex: 1, minHeight: 0 }}>
                        <List dense>
                            {rules.map((rule) => (
                                <ListItem key={rule.ID}>
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
                    </Box>
                </Box>
            )}
        </WidgetSwap>
    );
}
