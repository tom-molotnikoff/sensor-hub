import { Box, Typography } from '@mui/material';
import TuneIcon from '@mui/icons-material/Tune';
import { useWidgetStateReport } from './WidgetContext';

interface NeedsConfigurationProps {
    message?: string;
}

export default function NeedsConfiguration({ message = 'Configure this widget to get started.' }: NeedsConfigurationProps) {
    useWidgetStateReport('populated');

    return (
        <Box sx={{ display: 'flex', flexDirection: 'column', alignItems: 'center', justifyContent: 'center', height: '100%', gap: 1, p: 2 }}>
            <TuneIcon sx={{ fontSize: 40, color: 'text.disabled' }} />
            <Typography variant="body2" align="center" sx={{
                color: "text.secondary"
            }}>
                {message}
            </Typography>
        </Box>
    );
}
