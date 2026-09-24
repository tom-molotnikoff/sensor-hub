import { ThemeProvider } from '@mui/material';
import { render, screen } from '@testing-library/react';
import { MemoryRouter } from 'react-router';
import { act } from 'react';
import { describe, expect, it, vi } from 'vitest';
import AnchorStack from './AnchorStack';
import Bounded from './Bounded';
import Card from './Card';
import DashboardSlot from './DashboardSlot';
import ChartArea from './ChartArea';
import EmptyState from './EmptyState';
import Inline from './Inline';
import Metric, { MetricGroup } from './Metric';
import PageGrid from './PageGrid';
import Stack from './Stack';
import StandalonePage from './StandalonePage';
import Sticky, { StickyBar } from './Sticky';
import { theme } from './theme';
import { chartAreaHeight, emptyStateMinHeight } from './theme/tokens';

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
    expect(header).toHaveStyle({ display: 'flex', alignItems: 'center' });
    expect(header.lastElementChild).toHaveStyle({ marginLeft: 'auto' });
    expect(card.querySelector('[data-ui=card-body]')).toHaveTextContent('body');
    expect(screen.getByRole('heading', { name: 'Add Sensor' })).toHaveClass('MuiTypography-cardTitle');
  });

  it('renders a body even without a header', () => {
    const { container } = renderUi(<Card>body</Card>);

    expect(container.querySelector('[data-ui=card-header]')).toBeNull();
    expect(container.querySelector('[data-ui=card-body]')).toHaveTextContent('body');
  });
});

describe('Card in a bounded parent', () => {
  it('fills the parent without a title or surface of its own', () => {
    const { container } = renderUi(
      <Bounded>
        <Card title="Sensor Health">body</Card>
      </Bounded>,
    );

    const card = container.querySelector('[data-ui=card]')!;
    expect(card).toHaveStyle({ height: '100%' });
    expect(card.querySelector('[data-ui=card-header]')).toBeNull();
    expect(screen.queryByText('Sensor Health')).toBeNull();
    expect(card.querySelector('[data-ui=card-body]')).toHaveTextContent('body');
  });
});

describe('ChartArea', () => {
  it('declares its token height on a page', () => {
    const { container } = renderUi(<ChartArea size="md" />);

    const area = container.querySelector('[data-ui=chart-area]')!;
    expect(area).toHaveAttribute('data-ui-min-height', String(chartAreaHeight.md.compact));
  });

  it('fills a bounded parent and ignores its size', () => {
    const { container } = renderUi(
      <Bounded>
        <ChartArea size="lg" />
      </Bounded>,
    );

    const area = container.querySelector('[data-ui=chart-area]')!;
    expect(area).not.toHaveAttribute('data-ui-min-height');
    expect(area).toHaveStyle({ height: '100%' });
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

describe('Sticky', () => {
  it('sticks its offset below the app bar', () => {
    const { container } = renderUi(<Sticky offset={40}>rail</Sticky>);

    const sticky = container.querySelector('[data-ui=sticky]')!;
    expect(sticky).toHaveStyle({ position: 'sticky', top: `${56 + 40}px` });
  });

  it('keeps its own height inside a page grid item', () => {
    const { container } = renderUi(
      <PageGrid equalHeight>
        <PageGrid.Item>
          <Sticky>rail</Sticky>
        </PageGrid.Item>
      </PageGrid>,
    );

    expect(container.querySelector('[data-ui=sticky]')).toHaveStyle({ flexGrow: '0', flexShrink: '0' });
  });
});

describe('StickyBar', () => {
  it('puts the title before the actions and sticks right below the app bar', () => {
    const { container } = renderUi(<StickyBar title="Properties" actions={<button>Save</button>} />);

    const bar = container.querySelector('[data-ui=sticky-bar]')!;
    expect(bar).toHaveStyle({ position: 'sticky', top: '56px' });
    expect(bar.firstElementChild).toHaveTextContent('Properties');
    expect(bar.lastElementChild).toContainElement(screen.getByRole('button', { name: 'Save' }));
    expect(bar.lastElementChild).toHaveStyle({ marginLeft: 'auto' });
    expect(screen.getByRole('heading', { name: 'Properties' })).toHaveClass('MuiTypography-cardTitle');
  });
});

describe('AnchorStack', () => {
  function stubHeight(element: Element, height: number) {
    vi.spyOn(element, 'getBoundingClientRect').mockReturnValue({ height } as DOMRect);
  }

  function renderSections() {
    const { container } = renderUi(
      <AnchorStack landingOffset={100}>
        <section id="first">first</section>
        <section id="last">last</section>
      </AnchorStack>,
    );
    return {
      sections: container.querySelector('[data-ui=anchor-stack-sections]')!,
      room: container.querySelector('[data-ui=anchor-stack-room]')!,
    };
  }

  it('lands each section below the landing offset', () => {
    renderSections();

    expect(document.getElementById('last')).toHaveStyle({ scrollMarginTop: '100px' });
  });

  it('leaves room below the last section so it can reach the landing line', () => {
    const { sections, room } = renderSections();
    stubHeight(sections, 4000);
    stubHeight(document.getElementById('last')!, 300);

    act(() => { window.dispatchEvent(new Event('resize')); });

    expect(room).toHaveStyle({ height: `${window.innerHeight - 100 - 300}px` });
  });

  it('leaves no room when the sections already fit', () => {
    const { sections, room } = renderSections();
    stubHeight(sections, 100);
    stubHeight(document.getElementById('last')!, 50);

    act(() => { window.dispatchEvent(new Event('resize')); });

    expect(room).toHaveStyle({ height: '0px' });
  });
});

describe('Metric', () => {
  it('shows the value on the metric scale for its size, with its unit and label', () => {
    renderUi(<Metric value={20.72} unit="°C" label="Attic" size="lg" />);

    const value = screen.getByText('20.7');
    expect(value).toHaveClass('MuiTypography-metricLg');
    expect(value).toHaveTextContent('20.7°C');
    expect(screen.getByText('Attic')).toBeInTheDocument();
  });

  it('shows a dash when there is no value', () => {
    renderUi(<Metric value={null} label="No data available" />);

    expect(screen.getByText('—')).toHaveClass('MuiTypography-metricMd');
  });

  it('fills a bounded parent and starts from the smallest size', () => {
    const { container } = renderUi(
      <Bounded>
        <Metric value={20.7} size="lg" dial={{ percent: 50, tone: 'status.ok.strong' }} />
      </Bounded>,
    );

    expect(container.querySelector('[data-ui=metric]')).toHaveStyle({ height: '100%' });
    expect(screen.getByText('20.7')).toHaveClass('MuiTypography-metricSm');
    expect(container.querySelector('[data-ui=metric-dial]')).toBeInTheDocument();
  });

  it('lays a group of metrics side by side', () => {
    const { container } = renderUi(
      <MetricGroup>
        <Metric value={1} label="Min" />
        <Metric value={2} label="Max" />
      </MetricGroup>,
    );

    expect(container.querySelector('[data-ui=metric-group]')).toHaveStyle({ display: 'grid', gridAutoFlow: 'column' });
  });
});

describe('EmptyState', () => {
  it('keeps its title in the body variant at weight 600', () => {
    renderUi(<EmptyState title="Nothing here" />);

    const title = screen.getByText('Nothing here');
    expect(title).toHaveClass('MuiTypography-body1');
    expect(title).toHaveStyle({ fontWeight: '600' });
  });

  it('takes its default minimum height from the token', () => {
    const { container } = renderUi(<EmptyState title="Nothing here" />);

    expect(container.querySelector('[data-ui=empty-state]')).toHaveStyle({ minHeight: `${emptyStateMinHeight.md}px` });
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
      // @ts-expect-error StandalonePage takes no sx
      <StandalonePage title="t" sx={{ maxWidth: 600 }} />,
      // @ts-expect-error StandalonePage takes no style
      <StandalonePage title="t" style={{ maxWidth: 600 }} />,
      // @ts-expect-error StandalonePage takes no className
      <StandalonePage title="t" className="wide" />,
      // @ts-expect-error DashboardSlot takes no sx
      <DashboardSlot height={100} sx={{ height: 50 }} />,
      // @ts-expect-error DashboardSlot takes no style
      <DashboardSlot height={100} style={{ height: 50 }} />,
      // @ts-expect-error DashboardSlot needs a height
      <DashboardSlot />,
      // @ts-expect-error EmptyState takes no sx
      <EmptyState title="t" sx={{ minHeight: 0 }} />,
      // @ts-expect-error EmptyState takes no style
      <EmptyState title="t" style={{ minHeight: 0 }} />,
      // @ts-expect-error EmptyState takes no className
      <EmptyState title="t" className="short" />,
      // @ts-expect-error EmptyState takes no minHeight
      <EmptyState title="t" minHeight={120} />,
      // @ts-expect-error ChartArea needs a size
      <ChartArea />,
      // @ts-expect-error ChartArea takes no sx
      <ChartArea size="md" sx={{ height: 100 }} />,
      // @ts-expect-error ChartArea takes no style
      <ChartArea size="md" style={{ height: 100 }} />,
      // @ts-expect-error ChartArea takes no className
      <ChartArea size="md" className="tall" />,
      // @ts-expect-error ChartArea takes no height
      <ChartArea size="md" height={100} />,
      // @ts-expect-error Sticky takes no sx
      <Sticky sx={{ top: 0 }} />,
      // @ts-expect-error Sticky takes no style
      <Sticky style={{ top: 0 }} />,
      // @ts-expect-error Sticky takes no className
      <Sticky className="pinned" />,
      // @ts-expect-error StickyBar takes no sx
      <StickyBar title="t" sx={{ paddingY: 0 }} />,
      // @ts-expect-error StickyBar takes no style
      <StickyBar title="t" style={{ paddingY: 0 }} />,
      // @ts-expect-error StickyBar takes no className
      <StickyBar title="t" className="flat" />,
      // @ts-expect-error AnchorStack takes no sx
      <AnchorStack landingOffset={0} sx={{ gap: 0 }} />,
      // @ts-expect-error AnchorStack takes no style
      <AnchorStack landingOffset={0} style={{ gap: 0 }} />,
      // @ts-expect-error AnchorStack takes no className
      <AnchorStack landingOffset={0} className="tight" />,
      // @ts-expect-error Metric takes no sx
      <Metric value={1} sx={{ fontSize: 12 }} />,
      // @ts-expect-error Metric takes no style
      <Metric value={1} style={{ fontSize: 12 }} />,
      // @ts-expect-error Metric takes no className
      <Metric value={1} className="big" />,
      // @ts-expect-error MetricGroup takes no sx
      <MetricGroup sx={{ gap: 0 }} />,
      // @ts-expect-error MetricGroup takes no style
      <MetricGroup style={{ gap: 0 }} />,
      // @ts-expect-error MetricGroup takes no className
      <MetricGroup className="tight" />,
    ];
    expect(overrides).toHaveLength(50);
  });
});
