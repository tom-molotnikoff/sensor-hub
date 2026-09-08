import { Component, type ReactNode } from 'react';
import { Paper, Typography, Button, Box } from '@mui/material';
import WarningAmberIcon from '@mui/icons-material/WarningAmber';
import { useWidgetStateReport } from './WidgetContext';

interface Props {
    children: ReactNode;
    widgetId: string;
    onRemove?: (id: string) => void;
    onConfigure?: (id: string) => void;
}

interface State {
    hasError: boolean;
}

export class WidgetErrorBoundary extends Component<Props, State> {
    state: State = { hasError: false };

    static getDerivedStateFromError(): State {
        return { hasError: true };
    }

    render() {
        if (!this.state.hasError) return this.props.children;

        return (
            <WidgetErrorFallback
                widgetId={this.props.widgetId}
                onRemove={this.props.onRemove}
                onConfigure={this.props.onConfigure}
            />
        );
    }
}

type FallbackProps = Omit<Props, 'children'>;

function WidgetErrorFallback({ widgetId, onRemove, onConfigure }: FallbackProps) {
    useWidgetStateReport('error');

    return (
        <Paper sx={{ p: 2, height: '100%', display: 'flex', flexDirection: 'column', alignItems: 'center', justifyContent: 'center', gap: 1 }}>
            <WarningAmberIcon color="warning" sx={{ fontSize: 40 }} />
            <Typography variant="subtitle2" align="center" sx={{
                color: "text.secondary"
            }}>
                This widget encountered an error.
            </Typography>
            <Typography variant="caption" align="center" sx={{
                color: "text.secondary"
            }}>
                Try editing its configuration or removing it.
            </Typography>
            <Box sx={{ display: 'flex', gap: 1, mt: 1 }}>
                {onConfigure && (
                    <Button size="small" variant="outlined" onClick={() => onConfigure(widgetId)}>
                        Reconfigure
                    </Button>
                )}
                {onRemove && (
                    <Button size="small" color="error" onClick={() => onRemove(widgetId)}>
                        Remove
                    </Button>
                )}
            </Box>
        </Paper>
    );
}
