import { ThemeProvider } from '@mui/material';
import { render, screen } from '@testing-library/react';
import { MemoryRouter } from 'react-router';
import { describe, expect, it } from 'vitest';
import Card from './Card';
import EmptyState from './EmptyState';
import Inline from './Inline';
import PageGrid from './PageGrid';
import Stack from './Stack';
import { theme } from './theme';
import { emptyStateMinHeight } from './theme/tokens';

function renderUi(ui: React.ReactElement) {
  return render(
    <ThemeProvider theme={theme}>
      <MemoryRouter>{ui}</MemoryRouter>
    </ThemeProvider>,
  );
}

describe('Card', () => {
  it('puts the title before the actions in a header above its body', () => {
    const { container } = renderUi(
      <Card title="Add Sensor" actions={<button>Refresh</button>}>
        body
      </Card>,
    );

    const card = container.querySelector('[data-ui=card]')!;
    const header = card.querySelector('[data-ui=card-header]')!;
    expect(header.firstElementChild).toHaveTextContent('Add Sensor');
    expect(header.lastElementChild).toContainElement(screen.getByRole('button', { name: 'Refresh' }));
    expect(getComputedStyle(header).flexWrap).not.toBe('wrap');
    expect(card.querySelector('[data-ui=card-body]')).toHaveTextContent('body');
    expect(screen.getByRole('heading', { name: 'Add Sensor' })).toHaveClass('MuiTypography-cardTitle');
  });

  it('renders a body even without a header', () => {
    const { container } = renderUi(<Card>body</Card>);

    expect(container.querySelector('[data-ui=card-header]')).toBeNull();
    expect(container.querySelector('[data-ui=card-body]')).toHaveTextContent('body');
  });
});

describe('PageGrid', () => {
  it('renders its items in source order', () => {
    const { container } = renderUi(
      <PageGrid equalHeight>
        <PageGrid.Item span={{ wide: 8 }}>first</PageGrid.Item>
        <PageGrid.Item span={{ wide: 4 }}>second</PageGrid.Item>
      </PageGrid>,
    );

    const items = container.querySelectorAll('[data-ui=page-grid] > [data-ui=page-grid-item]');
    expect(Array.from(items, (item) => item.textContent)).toEqual(['first', 'second']);
  });
});

describe('EmptyState', () => {
  it('takes its default minimum height from the token', () => {
    const { container } = renderUi(<EmptyState title="Nothing here" />);

    expect(container.querySelector('[data-ui=empty-state]')).toHaveAttribute(
      'data-ui-min-height',
      String(emptyStateMinHeight.md),
    );
  });
});

describe('primitive props', () => {
  it('accept no styling or layout overrides', () => {
    const overrides = [
      // @ts-expect-error PageGrid takes no sx
      <PageGrid sx={{ gap: 0 }} />,
      // @ts-expect-error PageGrid takes no style
      <PageGrid style={{ gap: 0 }} />,
      // @ts-expect-error PageGrid takes no className
      <PageGrid className="tight" />,
      // @ts-expect-error PageGrid takes no column override
      <PageGrid columns={6} />,
      // @ts-expect-error PageGrid.Item takes no sx
      <PageGrid.Item sx={{ gridColumn: 'span 3' }} />,
      // @ts-expect-error PageGrid.Item takes no style
      <PageGrid.Item style={{ gridColumn: 'span 3' }} />,
      // @ts-expect-error PageGrid.Item takes no className
      <PageGrid.Item className="wide" />,
      // @ts-expect-error PageGrid.Item spans only on wide
      <PageGrid.Item span={{ compact: 6, wide: 6 }} />,
      // @ts-expect-error Card takes no sx
      <Card sx={{ padding: 0 }} />,
      // @ts-expect-error Card takes no style
      <Card style={{ padding: 0 }} />,
      // @ts-expect-error Card takes no className
      <Card className="tall" />,
      // @ts-expect-error Card takes no height
      <Card height={400} />,
      // @ts-expect-error Stack takes no sx
      <Stack sx={{ gap: 0 }} />,
      // @ts-expect-error Stack takes no style
      <Stack style={{ gap: 0 }} />,
      // @ts-expect-error Stack takes no className
      <Stack className="tight" />,
      // @ts-expect-error Stack takes no spacing
      <Stack spacing={2} />,
      // @ts-expect-error Inline takes no sx
      <Inline sx={{ flexWrap: 'nowrap' }} />,
      // @ts-expect-error Inline takes no style
      <Inline style={{ flexWrap: 'nowrap' }} />,
      // @ts-expect-error Inline takes no className
      <Inline className="nowrap" />,
      // @ts-expect-error Inline takes no wrap override
      <Inline wrap={false} />,
      // @ts-expect-error EmptyState takes no sx
      <EmptyState title="t" sx={{ minHeight: 0 }} />,
      // @ts-expect-error EmptyState takes no style
      <EmptyState title="t" style={{ minHeight: 0 }} />,
      // @ts-expect-error EmptyState takes no className
      <EmptyState title="t" className="short" />,
      // @ts-expect-error EmptyState takes no minHeight
      <EmptyState title="t" minHeight={120} />,
    ];
    expect(overrides).toHaveLength(24);
  });
});
