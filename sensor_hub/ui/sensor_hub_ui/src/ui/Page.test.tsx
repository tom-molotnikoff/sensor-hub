import { ThemeProvider } from '@mui/material';
import { render, screen, within } from '@testing-library/react';
import { MemoryRouter } from 'react-router';
import { afterEach, describe, expect, it, vi } from 'vitest';
import Page from './Page';
import { theme } from './theme';

function atWidth(width: number) {
  vi.stubGlobal('matchMedia', (query: string) => ({
    matches: Number(/\(min-width:\s*(\d+)px\)/.exec(query)?.[1] ?? 0) <= width,
    media: query,
    onchange: null,
    addListener: () => {},
    removeListener: () => {},
    addEventListener: () => {},
    removeEventListener: () => {},
    dispatchEvent: () => false,
  }));
}

function renderPage(page: React.ReactElement) {
  return render(
    <ThemeProvider theme={theme}>
      <MemoryRouter>{page}</MemoryRouter>
    </ThemeProvider>,
  );
}

const picker = <button type="button">Living room</button>;

describe('Page', () => {
  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it('renders a title element in the page header on the wide tier', () => {
    atWidth(1440);
    const { container } = renderPage(<Page title="Dashboards" titleElement={picker} actions={<button type="button">New</button>} />);

    const header = container.querySelector<HTMLElement>('[data-ui=page] > [data-ui=page-header]')!;
    expect(header).toBe(container.querySelector('[data-ui=page]')!.firstElementChild);
    const heading = within(header).getByRole('heading', { level: 1 });
    expect(heading).toContainElement(screen.getByRole('button', { name: 'Living room' }));
    expect(screen.getAllByRole('heading', { level: 1 })).toEqual([heading]);
    expect(within(header).getByRole('button', { name: 'New' })).toBeInTheDocument();
    expect(screen.queryByRole('banner')).not.toBeInTheDocument();
    expect(screen.queryByText('Dashboards')).not.toBeInTheDocument();
  });

  it('puts the before-title group ahead of the heading and the actions at the end of the wide header', () => {
    atWidth(1440);
    const { container } = renderPage(
      <Page
        title="Dashboards"
        titleElement={picker}
        beforeTitle={<button type="button">Lock</button>}
        actions={<button type="button">New</button>}
      />,
    );

    const header = container.querySelector<HTMLElement>('[data-ui=page-header]')!;
    const heading = within(header).getByRole('heading', { level: 1 });
    expect(heading).not.toContainElement(screen.getByRole('button', { name: 'Lock' }));
    expect(header.firstElementChild).toHaveAttribute('data-ui', 'page-before-title');
    expect(header.firstElementChild).toContainElement(screen.getByRole('button', { name: 'Lock' }));
    expect(heading.previousElementSibling).toBe(header.firstElementChild);
    expect(header.lastElementChild).toHaveAttribute('data-ui', 'page-actions');
    expect(header.lastElementChild).toContainElement(screen.getByRole('button', { name: 'New' }));
  });

  it('renders nothing before the title on the compact tier', () => {
    atWidth(390);
    renderPage(<Page title="Dashboards" beforeTitle={<button type="button">Lock</button>} />);

    expect(screen.queryByRole('button', { name: 'Lock' })).not.toBeInTheDocument();
  });

  it('renders the text title as the page header heading on the wide tier', () => {
    atWidth(1440);
    const { container } = renderPage(<Page title="Sensors Overview">content</Page>);

    const header = container.querySelector<HTMLElement>('[data-ui=page-header]')!;
    expect(within(header).getByRole('heading', { level: 1, name: 'Sensors Overview' })).toBeInTheDocument();
    expect(screen.getByRole('navigation', { name: 'Main' })).toBeInTheDocument();
  });

  it('renders the text title in the bar and no page header on the compact tier', () => {
    atWidth(390);
    const { container } = renderPage(<Page title="Dashboards" titleElement={picker} />);

    expect(within(screen.getByRole('banner')).getByRole('heading', { level: 1, name: 'Dashboards' })).toBeInTheDocument();
    expect(container.querySelector('[data-ui=page-header]')).toBeNull();
    expect(screen.queryByRole('button', { name: 'Living room' })).not.toBeInTheDocument();
  });

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
