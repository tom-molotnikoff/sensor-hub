import type { WidgetProps } from '../types';
import { Box, Typography } from '@mui/material';
import Markdown from 'react-markdown';
import { useWidgetStateReport } from '../WidgetContext';

export default function MarkdownNoteWidget({ config }: WidgetProps) {
    const content = (config.content as string) || '';
    useWidgetStateReport('populated');

    if (!content) {
        return (
            <Box sx={{ display: 'flex', alignItems: 'center', justifyContent: 'center', height: '100%', p: 2 }}>
                <Typography sx={{
                    color: "text.secondary"
                }}>Click settings to add content</Typography>
            </Box>
        );
    }

    return (
        <Box sx={{
            p: 2, overflow: 'auto', height: '100%',
            '& h1': { fontSize: '1.8rem', fontWeight: 700, mt: 0, mb: 1 },
            '& h2': { fontSize: '1.4rem', fontWeight: 600, mt: 1, mb: 0.5 },
            '& h3': { fontSize: '1.15rem', fontWeight: 600, mt: 1, mb: 0.5 },
            '& p': { mt: 0, mb: 1 },
            '& ul, & ol': { mt: 0, mb: 1, pl: 3 },
            '& code': { bgcolor: 'action.hover', px: 0.5, borderRadius: 0.5, fontFamily: 'monospace', fontSize: '0.85em' },
            '& pre': { bgcolor: 'action.hover', p: 1.5, borderRadius: 1, overflow: 'auto', '& code': { bgcolor: 'transparent', px: 0 } },
            '& blockquote': { borderLeft: 3, borderColor: 'primary.main', pl: 2, ml: 0, color: 'text.secondary' },
            '& a': { color: 'primary.main' },
            '& hr': { border: 'none', borderTop: 1, borderColor: 'divider', my: 1 },
            '& table': { borderCollapse: 'collapse', width: '100%', mb: 1 },
            '& th, & td': { border: 1, borderColor: 'divider', px: 1, py: 0.5 },
        }}>
            <Markdown>{content}</Markdown>
        </Box>
    );
}
