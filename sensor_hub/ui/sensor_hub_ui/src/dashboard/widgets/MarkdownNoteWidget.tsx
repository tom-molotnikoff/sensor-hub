import type { WidgetProps } from '../types';
import Markdown from 'react-markdown';
import { useWidgetStateReport } from '../WidgetContext';
import EmptyState from '../../ui/EmptyState';
import Prose from '../../ui/Prose';

export default function MarkdownNoteWidget({ config }: WidgetProps) {
    const content = (config.content as string) || '';
    useWidgetStateReport('populated');

    if (!content) {
        return <EmptyState size="sm" title="Click settings to add content" />;
    }

    return (
        <Prose>
            <Markdown>{content}</Markdown>
        </Prose>
    );
}
