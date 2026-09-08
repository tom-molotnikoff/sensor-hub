import { Box, Skeleton } from '@mui/material';
import { GRID_ROW_HEIGHT } from './constants';

const PLACEHOLDER_SPANS = [4, 4, 4, 6, 6, 12];

export default function DashboardSkeleton() {
    return (
        <Box
            data-testid="dashboard-skeleton"
            sx={{
                display: 'grid',
                gridTemplateColumns: { xs: 'repeat(4, 1fr)', md: 'repeat(12, 1fr)' },
                gap: 2,
            }}
        >
            {PLACEHOLDER_SPANS.map((span, index) => (
                <Skeleton
                    key={index}
                    variant="rectangular"
                    height={GRID_ROW_HEIGHT * 4}
                    sx={{ gridColumn: { xs: 'span 4', md: `span ${span}` }, borderRadius: 2 }}
                />
            ))}
        </Box>
    );
}
