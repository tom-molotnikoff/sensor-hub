import type { WidgetProps } from '../types';
import { List, ListItem, ListItemText, Typography } from '@mui/material';
import { useCurrentReadings, useCurrentReadingsReady } from '../../hooks/useCurrentReadings';
import NeedsConfiguration from '../NeedsConfiguration';
import { useReportWidgetUpdate } from '../WidgetUpdateContext';
import { useWidgetStateReport } from '../WidgetContext';
import { WidgetSwap, ValuePlaceholderLoader } from '../../ui/loaders';
import Card from '../../ui/Card';
import Metric from '../../ui/Metric';

export default function GroupSummaryWidget({ config }: WidgetProps) {
    const reportUpdate = useReportWidgetUpdate();
    const readings = useCurrentReadings({ onDataUpdate: reportUpdate });
    const ready = useCurrentReadingsReady();
    useWidgetStateReport(ready ? 'populated' : 'loading');
    const measurementType = config.measurementType as string | undefined;

    if (!measurementType) {
        return <NeedsConfiguration message="Select a measurement type" />;
    }

    // Collect the reading for the configured measurement type from each sensor
    const matched: { name: string; value: number | null; unit: string }[] = [];
    for (const [name, byType] of Object.entries(readings)) {
        const reading = byType[measurementType];
        if (reading) {
            matched.push({ name, value: reading.numeric_value, unit: reading.unit });
        }
    }

    const isLoading = matched.length === 0 && !ready;
    const nums = matched.filter(r => r.value !== null).map(r => r.value!);
    const avg = nums.length > 0 ? nums.reduce((sum, v) => sum + v, 0) / nums.length : 0;
    const unit = matched[0]?.unit ?? '';

    return (
        <WidgetSwap loading={isLoading} loader={<ValuePlaceholderLoader />}>
            {matched.length === 0 ? (
                <Metric value={null} label={`No ${measurementType} readings available`} />
            ) : (
                <Card>
                    <Metric value={avg} unit={unit} label="Group Average" />
                    <List dense disablePadding>
                        {matched.map(({ name, value, unit: u }) => (
                            <ListItem
                                key={name}
                                secondaryAction={<Typography variant="caption">{value?.toFixed(1) ?? '—'}{u}</Typography>}
                            >
                                <ListItemText primary={name} slotProps={{ primary: { variant: 'caption', color: 'text.secondary' } }} />
                            </ListItem>
                        ))}
                    </List>
                </Card>
            )}
        </WidgetSwap>
    );
}
