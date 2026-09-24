import { Component, type ReactNode } from 'react';
import { Button } from '@mui/material';
import WarningAmberIcon from '@mui/icons-material/WarningAmber';
import EmptyState from '../ui/EmptyState';
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
        <EmptyState
            size="sm"
            icon={<WarningAmberIcon color="warning" fontSize="large" />}
            title="This widget encountered an error."
            description="Try editing its configuration or removing it."
            actions={(onConfigure || onRemove) && (
                <>
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
                </>
            )}
        />
    );
}
