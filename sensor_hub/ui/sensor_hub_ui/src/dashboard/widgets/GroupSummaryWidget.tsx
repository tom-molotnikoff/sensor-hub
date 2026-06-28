import type { WidgetProps } from '../types';
import { Box, Typography } from '@mui/material';
import { useCurrentReadings, useCurrentReadingsReady } from '../../hooks/useCurrentReadings';
import NeedsConfiguration from '../NeedsConfiguration';
import { useReportWidgetUpdate } from '../WidgetUpdateContext';
import { WidgetSwap, ValuePlaceholderLoader } from '../widget-loaders';

export default function GroupSummaryWidget({ config }: WidgetProps) {
    const reportUpdate = useReportWidgetUpdate();
    const readings = useCurrentReadings({ onDataUpdate: reportUpdate });
    const ready = useCurrentReadingsReady();
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
                <Box sx={{ display: 'flex', alignItems: 'center', justifyContent: 'center', height: '100%' }}>
                    <Typography sx={{
                        color: "text.secondary"
                    }}>No {measurementType} readings available</Typography>
                </Box>
            ) : (
                <Box sx={{ display: 'flex', flexDirection: 'column', height: '100%', p: 2 }}>
                    <Typography variant="subtitle1" sx={{ fontWeight: 'bold' }}>Group Average</Typography>
                    <Box sx={{ flex: 1, display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
                        <Typography variant="h2" sx={{ fontWeight: 'bold', textAlign: 'center' }}>
                            {avg.toFixed(1)}{unit}
                        </Typography>
                    </Box>
                    <Box sx={{ maxHeight: 120, overflow: 'auto' }}>
                        {matched.map(({ name, value, unit: u }) => (
                            <Box key={name} sx={{ display: 'flex', justifyContent: 'space-between', py: 0.25 }}>
                                <Typography variant="caption" sx={{
                                    color: "text.secondary"
                                }}>{name}</Typography>
                                <Typography variant="caption">{value?.toFixed(1) ?? '—'}{u}</Typography>
                            </Box>
                        ))}
                    </Box>
                </Box>
            )}
        </WidgetSwap>
    );
}
