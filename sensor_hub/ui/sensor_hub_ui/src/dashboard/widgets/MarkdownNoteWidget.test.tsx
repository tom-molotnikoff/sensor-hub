import { ThemeProvider } from '@mui/material';
import { render, screen } from '@testing-library/react';
import { MemoryRouter } from 'react-router';
import { describe, expect, it } from 'vitest';
import Bounded from '../../ui/Bounded';
import { theme } from '../../ui/theme';
import MarkdownNoteWidget from './MarkdownNoteWidget';

function renderNote(content?: string) {
  return render(
    <ThemeProvider theme={theme}>
      <MemoryRouter>
        <Bounded>
          <MarkdownNoteWidget id="w" isEditing={false} config={content === undefined ? {} : { content }} />
        </Bounded>
      </MemoryRouter>
    </ThemeProvider>,
  );
}

describe('MarkdownNoteWidget', () => {
  it('asks for content when it has none', () => {
    renderNote();

    expect(screen.getByText('Click settings to add content')).toBeInTheDocument();
  });

  it('renders its markdown as prose that fills the frame', () => {
    const { container } = renderNote('# Plants\n\nWater the **basil** on [Mondays](https://example.com).');

    expect(screen.getByRole('heading', { level: 1, name: 'Plants' })).toBeInTheDocument();
    expect(screen.getByText('basil').tagName).toBe('STRONG');
    expect(screen.getByRole('link', { name: 'Mondays' })).toHaveAttribute('href', 'https://example.com');
    expect(container.querySelector('[data-ui=prose]')).toHaveStyle({ height: '100%', overflow: 'auto' });
  });
});
