import { useEffect, useState } from 'react';
import { Tooltip, Typography } from '@mui/material';

function formatRelative(date: Date): string {
    const seconds = Math.round((Date.now() - date.getTime()) / 1000);
    if (seconds < 5) return 'just now';
    if (seconds < 60) return `${seconds}s ago`;
    const minutes = Math.floor(seconds / 60);
    if (minutes < 60) return `${minutes}m ago`;
    const hours = Math.floor(minutes / 60);
    if (hours < 24) return `${hours}h ago`;
    const days = Math.floor(hours / 24);
    return `${days}d ago`;
}

interface RelativeTimeProps {
    date: Date;
}

/** Displays a self-updating relative timestamp with an absolute tooltip. */
export default function RelativeTime({ date }: RelativeTimeProps) {
    const [, setTick] = useState(0);

    useEffect(() => {
        const id = window.setInterval(() => setTick((t) => t + 1), 30_000);
        return () => window.clearInterval(id);
    }, []);

    return (
        <Tooltip title={date.toLocaleString()} arrow>
            <Typography variant="caption" color="text.disabled" noWrap sx={{ cursor: 'default' }}>
                {formatRelative(date)}
            </Typography>
        </Tooltip>
    );
}
