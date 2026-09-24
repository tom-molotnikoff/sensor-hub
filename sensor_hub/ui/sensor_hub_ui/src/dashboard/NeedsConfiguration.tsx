import TuneIcon from '@mui/icons-material/Tune';
import EmptyState from '../ui/EmptyState';
import { useWidgetStateReport } from './WidgetContext';

interface NeedsConfigurationProps {
    message?: string;
}

export default function NeedsConfiguration({ message = 'Configure this widget to get started.' }: NeedsConfigurationProps) {
    useWidgetStateReport('populated');

    return <EmptyState size="sm" icon={<TuneIcon fontSize="large" />} title={message} />;
}
