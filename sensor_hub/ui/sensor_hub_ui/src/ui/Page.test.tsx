import { ThemeProvider } from '@mui/material';
import { render, screen } from '@testing-library/react';
import { MemoryRouter } from 'react-router';
import { describe, expect, it } from 'vitest';
import Page from './Page';
import { theme } from './theme';

function renderPage(page: React.ReactElement) {
  return render(
    <ThemeProvider theme={theme}>
      <MemoryRouter>{page}</MemoryRouter>
    </ThemeProvider>,
  );
}

describe('Page', () => {
  it('renders the app bar title and its content inside the page root', () => {
    const { container } = renderPage(<Page title="Sensors Overview">content</Page>);

    expect(screen.getByText('Sensors Overview')).toBeInTheDocument();
    expect(container.querySelector('[data-ui=page]')).toHaveTextContent('content');
  });

  it('shows a progress indicator instead of its content while loading', () => {
    const { container } = renderPage(<Page title="Sensors Overview" loading>content</Page>);

    expect(screen.getByRole('progressbar')).toBeInTheDocument();
    expect(container.querySelector('[data-ui=page]')).not.toHaveTextContent('content');
  });

  it('accepts no styling overrides', () => {
    const overrides = [
      // @ts-expect-error Page takes no sx
      <Page title="t" sx={{ padding: 0 }} />,
      // @ts-expect-error Page takes no style
      <Page title="t" style={{ padding: 0 }} />,
      // @ts-expect-error Page takes no className
      <Page title="t" className="wide" />,
    ];
    expect(overrides).toHaveLength(3);
  });
});
