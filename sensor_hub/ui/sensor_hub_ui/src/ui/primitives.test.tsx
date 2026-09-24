import { ThemeProvider } from '@mui/material';
import { fireEvent, render, screen } from '@testing-library/react';
import { MemoryRouter } from 'react-router';
import { act } from 'react';
import { describe, expect, it, vi } from 'vitest';
import ActionBar from './ActionBar';
import AnchorStack from './AnchorStack';
import Bounded from './Bounded';
import Card from './Card';
import DashboardCanvas from './DashboardCanvas';
import DashboardSlot from './DashboardSlot';
import ChartArea from './ChartArea';
import DataTable from './DataTable';
import EmptyState from './EmptyState';
import Frame from './Frame';
import Inline from './Inline';
import MenuPanel from './MenuPanel';
import Metric, { MetricGroup } from './Metric';
import PageGrid from './PageGrid';
import Prose from './Prose';
import Stack from './Stack';
import StandalonePage from './StandalonePage';
import StatGrid from './StatGrid';
import Strip, { StripCell, StripDetail } from './Strip';
import SlideSwitch from './SlideSwitch';
import Sticky, { StickyBar } from './Sticky';
import TileGrid from './TileGrid';
import { theme } from './theme';
import { chartAreaHeight, emptyStateMinHeight, stripNarrowWidth } from './theme/tokens';

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

describe('Card inset in a bounded parent', () => {
  it('insets raw content once', () => {
    const { container } = renderUi(
      <Bounded>
        <Card>
          <p>raw</p>
          <Metric value={1} />
        </Card>
      </Bounded>,
    );

    expect(container.querySelector('[data-ui=card-body]')).toHaveAttribute('data-ui-inset', 'true');
    expect(container.querySelector('[data-ui=metric]')).toHaveStyle({ padding: '0px' });
  });

  it('leaves the inset to a table or chart however deeply it is wrapped', () => {
    const { container } = renderUi(
      <Bounded>
        <Card>
          <div>
            <DataTable rows={[{ id: 1, name: 'a' }]} columns={[{ field: 'name', compact: 'title' }]} />
          </div>
        </Card>
        <Card>
          <section>
            <ChartArea size="md" placeholder={<span>loading</span>} />
          </section>
        </Card>
      </Bounded>,
    );

    for (const body of container.querySelectorAll('[data-ui=card-body]')) {
      expect(body).not.toHaveAttribute('data-ui-inset');
    }
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

describe('Frame', () => {
  it('is a flat surface with its title in the caption variant', () => {
    const { container } = renderUi(<Frame title="Gauge: attic">body</Frame>);

    const frame = container.querySelector('[data-ui=frame]')!;
    expect(frame).toHaveStyle({ boxShadow: 'none', borderStyle: 'solid' });
    expect(screen.getByText('Gauge: attic')).toHaveClass('MuiTypography-caption');
    expect(container.querySelector('[data-ui=frame-body]')).toHaveTextContent('body');
  });

  it('bounds its body, so a card inside fills it', () => {
    const { container } = renderUi(
      <Frame title="Sensor Health">
        <Card title="Sensor Health">body</Card>
      </Frame>,
    );

    expect(container.querySelector('[data-ui=card]')).toHaveStyle({ height: '100%' });
    expect(container.querySelector('[data-ui=card-header]')).toBeNull();
  });

  it('marks its header as the drag handle only while editing a draggable frame', () => {
    const { container, rerender } = renderUi(<Frame title="t" dragHandle>body</Frame>);
    expect(container.querySelector('.drag-handle')).toBeNull();

    rerender(
      <ThemeProvider theme={theme}>
        <MemoryRouter>
          <Frame title="t" dragHandle editing>
            body
          </Frame>
        </MemoryRouter>
      </ThemeProvider>,
    );
    expect(container.querySelector('[data-ui=frame-header]')).toHaveClass('drag-handle');
    expect(container.querySelector('[data-ui=frame]')).toHaveStyle({ borderStyle: 'dashed' });
  });
});

describe('EmptyState', () => {
  it('fills a bounded parent instead of taking its minimum height', () => {
    const { container } = renderUi(
      <Bounded>
        <EmptyState title="Nothing here" />
      </Bounded>,
    );

    const state = container.querySelector('[data-ui=empty-state]');
    expect(state).toHaveStyle({ height: '100%' });
    expect(state).not.toHaveStyle({ minHeight: `${emptyStateMinHeight.md}px` });
  });

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

describe('ActionBar', () => {
  it('wraps its actions, keeps the picker a usable width, and pushes trailing actions to the far edge', () => {
    const { container } = renderUi(
      <ActionBar picker={<select aria-label="Dashboard" />} trailing={<button>New</button>}>
        <button>Edit</button>
      </ActionBar>,
    );

    const bar = container.querySelector('[data-ui=action-bar]')!;
    expect(bar).toHaveStyle({ display: 'flex', flexWrap: 'wrap' });
    expect(bar.firstElementChild).toHaveAttribute('data-ui', 'action-bar-picker');
    expect(bar.firstElementChild).toContainElement(screen.getByRole('combobox', { name: 'Dashboard' }));
    expect(bar.firstElementChild).toHaveStyle({ minWidth: 'min(200px, 100%)' });
    const trailing = bar.lastElementChild!;
    expect(trailing).toHaveAttribute('data-ui', 'action-bar-trailing');
    expect(trailing).toContainElement(screen.getByRole('button', { name: 'New' }));
    expect(trailing).toHaveStyle({ marginLeft: 'auto', flexWrap: 'wrap' });
  });

  it('renders only the actions when there is no picker or trailing group', () => {
    const { container } = renderUi(
      <ActionBar>
        <span>View only</span>
      </ActionBar>,
    );

    expect(container.querySelector('[data-ui=action-bar-picker]')).toBeNull();
    expect(container.querySelector('[data-ui=action-bar-trailing]')).toBeNull();
    expect(container.querySelector('[data-ui=action-bar]')).toHaveTextContent('View only');
  });
});

describe('DashboardCanvas', () => {
  it('leaves room below the grid only while editing and exposes its element', () => {
    const ref = { current: null as HTMLDivElement | null };
    const { container, rerender } = renderUi(<DashboardCanvas ref={ref} editing={false}>grid</DashboardCanvas>);

    const canvas = container.querySelector('[data-ui=dashboard-canvas]')!;
    expect(ref.current).toBe(canvas);
    expect(canvas).toHaveStyle({ paddingBottom: '0px' });

    rerender(
      <ThemeProvider theme={theme}>
        <MemoryRouter>
          <DashboardCanvas ref={ref} editing>
            grid
          </DashboardCanvas>
        </MemoryRouter>
      </ThemeProvider>,
    );
    expect(container.querySelector('[data-ui=dashboard-canvas]')).toHaveStyle({ paddingBottom: '200px' });
  });
});

describe('Prose', () => {
  it('sets its text in the body variant and headings on the type scale', () => {
    const { container } = renderUi(
      <Prose>
        <h1>Plants</h1>
        <p>Water the basil</p>
      </Prose>,
    );

    const prose = container.querySelector('[data-ui=prose]')!;
    expect(prose).toHaveStyle({ fontSize: theme.typography.body.fontSize });
    expect(screen.getByRole('heading', { name: 'Plants' })).toHaveStyle({ fontWeight: theme.typography.cardTitle.fontWeight });
    expect(prose).not.toHaveStyle({ height: '100%' });
  });

  it('fills and scrolls a bounded parent', () => {
    const { container } = renderUi(
      <Bounded>
        <Prose>
          <p>Water the basil</p>
        </Prose>
      </Bounded>,
    );

    expect(container.querySelector('[data-ui=prose]')).toHaveStyle({ height: '100%', overflow: 'auto' });
  });
});

describe('TileGrid', () => {
  const tiles = [
    { key: 1, label: 1, fill: 'status.ok.strong', ink: 'common.white' },
    { key: 2, label: 2, fill: 'status.bad.strong', ink: 'common.white' },
    { key: 3, label: 3, fill: 'action.hover', ink: 'text.secondary' },
  ];

  it('shows one tile per entry, in order, under its caption', () => {
    const { container } = renderUi(<TileGrid columns={7} tiles={tiles} caption="September" />);

    const grid = container.querySelector('[data-ui=tile-grid]')!;
    expect(grid.firstElementChild).toHaveTextContent('September');
    expect([...grid.querySelectorAll('[data-ui=tile]')].map((tile) => tile.textContent)).toEqual(['1', '2', '3']);
    expect(grid).not.toHaveStyle({ height: '100%' });
  });

  it('fills a bounded parent', () => {
    const { container } = renderUi(
      <Bounded>
        <TileGrid columns={7} tiles={tiles} />
      </Bounded>,
    );

    expect(container.querySelector('[data-ui=tile-grid]')).toHaveStyle({ height: '100%' });
  });
});

describe('SlideSwitch', () => {
  it('reports the flipped state when clicked', () => {
    const onChange = vi.fn();
    renderUi(<SlideSwitch checked={false} label="Toggle plug" onChange={onChange} />);

    fireEvent.click(screen.getByRole('checkbox', { name: 'Toggle plug' }));

    expect(onChange).toHaveBeenCalledWith(true);
  });

  it('stays mixed and inert until its state is known, and inert when read only', () => {
    const onChange = vi.fn();
    const { rerender } = renderUi(<SlideSwitch checked={null} label="Toggle plug" onChange={onChange} />);
    const control = screen.getByRole('checkbox', { name: 'Toggle plug' });
    expect(control).toHaveAttribute('aria-checked', 'mixed');
    fireEvent.click(control);

    rerender(
      <ThemeProvider theme={theme}>
        <MemoryRouter>
          <SlideSwitch checked label="Toggle plug" readOnly onChange={onChange} />
        </MemoryRouter>
      </ThemeProvider>,
    );
    expect(control).toHaveAttribute('aria-disabled', 'true');
    fireEvent.click(control);

    expect(onChange).not.toHaveBeenCalled();
  });

  it('fills a bounded parent and centres the switch in it', () => {
    const { container } = renderUi(
      <Bounded>
        <SlideSwitch checked label="Toggle plug" onChange={() => {}} />
      </Bounded>,
    );

    expect(container.querySelector('[data-ui=slide-switch]')).toHaveStyle({ height: '100%', alignItems: 'center' });
  });
});

describe('StatGrid', () => {
  it('shows each stat as a label above its value, in order', () => {
    const { container } = renderUi(
      <StatGrid
        stats={[
          { key: 'temperature', label: 'Temperature', value: '21.5 °C' },
          { key: 'humidity', label: 'Humidity', value: '48.0 %' },
        ]}
      />,
    );

    const stats = [...container.querySelectorAll('[data-ui=stat-grid] > [data-ui=stat]')];
    expect(stats.map((stat) => [stat.firstElementChild?.textContent, stat.lastElementChild?.textContent])).toEqual([
      ['Temperature', '21.5 °C'],
      ['Humidity', '48.0 %'],
    ]);
    expect(container.querySelector('[data-ui=stat-grid]')).toHaveStyle({ display: 'grid' });
  });
});

describe('MenuPanel', () => {
  function anchor() {
    const button = document.createElement('button');
    document.body.appendChild(button);
    button.focus();
    return button;
  }

  it('stays closed without an anchor', () => {
    renderUi(<MenuPanel anchorEl={null} onClose={() => {}} title="Notifications" />);

    expect(screen.queryByText('Notifications')).toBeNull();
  });

  it('shows its title and meta above the body, and the footer below', () => {
    renderUi(
      <MenuPanel anchorEl={anchor()} onClose={() => {}} title="Notifications" meta="3 unread" footer={<button>View all</button>}>
        <li>row</li>
      </MenuPanel>,
    );

    const header = document.querySelector('[data-ui=menu-panel-header]')!;
    expect(header.firstElementChild).toHaveTextContent('Notifications');
    expect(header.lastElementChild).toHaveTextContent('3 unread');
    expect(screen.getByText('row')).toBeInTheDocument();
    expect(document.querySelector('[data-ui=menu-panel-footer]')).toContainElement(screen.getByRole('button', { name: 'View all' }));
  });

  it('shows a spinner instead of the body while loading', () => {
    renderUi(
      <MenuPanel anchorEl={anchor()} onClose={() => {}} title="Notifications" loading>
        <li>row</li>
      </MenuPanel>,
    );

    expect(screen.getByRole('status', { name: 'Loading' })).toBeInTheDocument();
    expect(screen.queryByText('row')).toBeNull();
    expect(document.querySelector('[data-ui=menu-panel-footer]')).toBeNull();
  });
});

describe('Strip', () => {
  it('lays its cells out in a labelled row that scrolls on its own and sheds detail when narrow', () => {
    const { container } = renderUi(
      <Strip label="Daily forecast" size="md">
        <StripCell highlighted>
          Mon<StripDetail>1 Jan</StripDetail>
        </StripCell>
        <StripCell>Tue</StripCell>
      </Strip>,
    );

    const strip = container.querySelector('[data-ui=strip]')!;
    const track = screen.getByRole('region', { name: 'Daily forecast' });
    expect(strip).toHaveStyle({ containerType: 'inline-size', minWidth: '0px' });
    expect(track).toHaveStyle({ display: 'flex', overflowX: 'auto' });
    expect(track).toHaveAttribute('tabindex', '0');
    const cells = track.querySelectorAll('[data-ui=strip-cell]');
    expect(cells).toHaveLength(2);
    expect(cells[0]).toHaveAttribute('aria-current', 'true');
    expect(cells[1]).not.toHaveAttribute('aria-current');
    expect(cells[0].querySelector('[data-ui=strip-detail]')).toHaveTextContent('1 Jan');
    const rules = Array.from(document.styleSheets).flatMap((sheet) => Array.from(sheet.cssRules).map((rule) => rule.cssText));
    expect(rules.some((rule) => rule.includes(`max-width: ${stripNarrowWidth - 0.05}px`) && rule.includes('strip-detail'))).toBe(true);
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
      <StandalonePage sx={{ maxWidth: 600 }} />,
      // @ts-expect-error StandalonePage takes no style
      <StandalonePage style={{ maxWidth: 600 }} />,
      // @ts-expect-error StandalonePage takes no className
      <StandalonePage className="wide" />,
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
      // @ts-expect-error Frame takes no sx
      <Frame sx={{ height: 100 }} />,
      // @ts-expect-error Frame takes no style
      <Frame style={{ height: 100 }} />,
      // @ts-expect-error Frame takes no className
      <Frame className="raised" />,
      // @ts-expect-error ActionBar takes no sx
      <ActionBar sx={{ gap: 0 }} />,
      // @ts-expect-error ActionBar takes no style
      <ActionBar style={{ gap: 0 }} />,
      // @ts-expect-error ActionBar takes no className
      <ActionBar className="tight" />,
      // @ts-expect-error DashboardCanvas takes no sx
      <DashboardCanvas editing sx={{ paddingBottom: 0 }} />,
      // @ts-expect-error DashboardCanvas takes no style
      <DashboardCanvas editing style={{ paddingBottom: 0 }} />,
      // @ts-expect-error DashboardCanvas takes no className
      <DashboardCanvas editing className="tall" />,
      // @ts-expect-error DashboardCanvas needs to know whether it is editing
      <DashboardCanvas />,
      // @ts-expect-error Prose takes no sx
      <Prose sx={{ padding: 0 }} />,
      // @ts-expect-error Prose takes no style
      <Prose style={{ padding: 0 }} />,
      // @ts-expect-error Prose takes no className
      <Prose className="wide" />,
      // @ts-expect-error TileGrid takes no sx
      <TileGrid columns={7} tiles={[]} sx={{ gap: 0 }} />,
      // @ts-expect-error TileGrid takes no style
      <TileGrid columns={7} tiles={[]} style={{ gap: 0 }} />,
      // @ts-expect-error TileGrid takes no className
      <TileGrid columns={7} tiles={[]} className="tight" />,
      // @ts-expect-error SlideSwitch takes no sx
      <SlideSwitch checked label="t" onChange={() => {}} sx={{ width: 100 }} />,
      // @ts-expect-error SlideSwitch takes no style
      <SlideSwitch checked label="t" onChange={() => {}} style={{ width: 100 }} />,
      // @ts-expect-error SlideSwitch takes no className
      <SlideSwitch checked label="t" onChange={() => {}} className="wide" />,
      // @ts-expect-error StatGrid takes no sx
      <StatGrid stats={[]} sx={{ gap: 0 }} />,
      // @ts-expect-error StatGrid takes no style
      <StatGrid stats={[]} style={{ gap: 0 }} />,
      // @ts-expect-error StatGrid takes no className
      <StatGrid stats={[]} className="tight" />,
      // @ts-expect-error MenuPanel takes no sx
      <MenuPanel anchorEl={null} onClose={() => {}} title="t" sx={{ width: 100 }} />,
      // @ts-expect-error MenuPanel takes no style
      <MenuPanel anchorEl={null} onClose={() => {}} title="t" style={{ width: 100 }} />,
      // @ts-expect-error MenuPanel takes no className
      <MenuPanel anchorEl={null} onClose={() => {}} title="t" className="wide" />,
      // @ts-expect-error Strip takes no sx
      <Strip label="t" size="md" sx={{ gap: 0 }} />,
      // @ts-expect-error Strip takes no style
      <Strip label="t" size="md" style={{ gap: 0 }} />,
      // @ts-expect-error Strip takes no className
      <Strip label="t" size="md" className="tight" />,
      // @ts-expect-error Strip needs a label for its scroll region
      <Strip size="md" />,
      // @ts-expect-error StripCell takes no sx
      <StripCell sx={{ padding: 0 }} />,
      // @ts-expect-error StripCell takes no style
      <StripCell style={{ padding: 0 }} />,
      // @ts-expect-error StripCell takes no className
      <StripCell className="flat" />,
      // @ts-expect-error StripDetail takes no sx
      <StripDetail sx={{ display: 'block' }} />,
    ];
    expect(overrides).toHaveLength(83);
  });
});
