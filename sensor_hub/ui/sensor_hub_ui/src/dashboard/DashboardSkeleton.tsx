import { Skeleton } from '@mui/material';
import PageGrid from '../ui/PageGrid';
import { GRID_ROW_HEIGHT } from './constants';

const PLACEHOLDER_SPANS = [4, 4, 4, 6, 6, 12] as const;

export default function DashboardSkeleton() {
    return (
        <div data-testid="dashboard-skeleton">
            <PageGrid>
                {PLACEHOLDER_SPANS.map((span, index) => (
                    <PageGrid.Item key={index} span={{ wide: span }}>
                        <Skeleton variant="rectangular" height={GRID_ROW_HEIGHT * 4} sx={{ borderRadius: 2 }} />
                    </PageGrid.Item>
                ))}
            </PageGrid>
        </div>
    );
}
