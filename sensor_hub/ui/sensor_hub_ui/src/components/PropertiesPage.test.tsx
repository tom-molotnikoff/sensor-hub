import { act, fireEvent, render, screen, waitFor, within } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import type { PropertyDefinitionsResponse } from '../gen/aliases';
import { FakeWebSocket, installFakeWebSocket } from '../test/fakeWebSocket';

class FakeIntersectionObserver {
  static instances: FakeIntersectionObserver[] = [];
  private readonly targets: Element[] = [];
  private readonly callback: IntersectionObserverCallback;

  constructor(callback: IntersectionObserverCallback) {
    this.callback = callback;
    FakeIntersectionObserver.instances.push(this);
  }

  observe(target: Element) {
    this.targets.push(target);
  }

  unobserve() {}

  disconnect() {}

  crossInto(ids: string[]) {
    const entries = this.targets.map((target) => ({
      target,
      isIntersecting: ids.includes(target.id),
    }));
    this.callback(entries as unknown as IntersectionObserverEntry[], this as never);
  }
}

vi.setConfig({ testTimeout: 10000 });

const { getMock, patchMock } = vi.hoisted(() => ({
  getMock: vi.fn(),
  patchMock: vi.fn(),
}));

vi.mock('../gen/client', () => ({
  apiClient: {
    GET: getMock,
    PATCH: patchMock,
  },
}));

const definitionsResponse: PropertyDefinitionsResponse = {
  definitions: [
    {
      key: 'sensor.discovery.skip',
      label: 'Skip sensor discovery',
      description: "Don't try to auto-discover sensors at startup.",
      type: 'bool',
      default: 'false',
      group: 'sensors',
      apply: 'live',
      readOnly: false,
    },
    {
      key: 'sensor.collection.interval',
      label: 'Collection interval',
      description: 'How often every enabled sensor is polled.',
      type: 'int',
      default: '300',
      group: 'sensors',
      unit: 'seconds',
      apply: 'next-cycle',
      readOnly: false,
    },
    {
      key: 'database.path',
      label: 'Database file',
      description: 'SQLite database file.',
      type: 'string',
      default: 'data/sensor_hub.db',
      group: 'advanced',
      apply: 'readonly',
      readOnly: true,
    },
  ],
  groups: [
    { id: 'advanced', label: 'Advanced', description: 'Rarely-changed settings.', order: 2 },
    { id: 'sensors', label: 'Sensors & collection', description: 'How often sensors are polled.', order: 1 },
  ],
};

const serverValues: Record<string, string> = {
  'sensor.discovery.skip': 'true',
  'sensor.collection.interval': '300',
  'database.path': '/var/lib/sensor-hub/sensor_hub.db',
};

let restoreWebSocket: () => void;

const nativeIntersectionObserver = globalThis.IntersectionObserver;
const nativeInnerWidth = window.innerWidth;

function setViewportWidth(width: number) {
  Object.defineProperty(window, 'innerWidth', { configurable: true, value: width });
}

async function renderPageUntil(permissions: string[], settled: string) {
  // Fresh imports per test so the session cache in usePropertyDefinitions is empty,
  // and so the AuthContext instance matches the one the page imports.
  const { default: PropertiesPage } = await import('./PropertiesPage');
  const { AuthContext } = await import('../providers/AuthContext');

  render(
    <AuthContext.Provider value={{ user: { id: 1, username: 'owner', roles: [], permissions }, refresh: async () => {} }}>
      <PropertiesPage />
    </AuthContext.Provider>,
  );

  await waitFor(() => expect(FakeWebSocket.instances.length).toBe(1));
  act(() => {
    FakeWebSocket.instances[0].serverSends(JSON.stringify(serverValues));
  });
  await waitFor(() => expect(screen.getByText(settled)).toBeInTheDocument());
}

async function renderPage(permissions: string[]) {
  await renderPageUntil(permissions, 'Skip sensor discovery');
}

async function renderFallbackPage(permissions: string[]) {
  await renderPageUntil(permissions, 'sensor.discovery.skip');
}

describe('PropertiesPage', () => {
  beforeEach(() => {
    getMock.mockReset();
    patchMock.mockReset();
    vi.resetModules();
    restoreWebSocket = installFakeWebSocket();
    getMock.mockResolvedValue({ data: definitionsResponse });
    patchMock.mockResolvedValue({ data: { message: 'ok' } });
    FakeIntersectionObserver.instances = [];
    globalThis.IntersectionObserver = FakeIntersectionObserver as never;
    window.location.hash = '';
  });

  afterEach(() => {
    restoreWebSocket();
    globalThis.IntersectionObserver = nativeIntersectionObserver;
    setViewportWidth(nativeInnerWidth);
    window.location.hash = '';
  });

  it('renders a field for each definition carrying the current value from the value feed', async () => {
    await renderPage(['view_properties', 'manage_properties']);

    expect(screen.getByRole('switch', { name: 'Skip sensor discovery' })).toBeChecked();
    expect(screen.getByRole('spinbutton', { name: 'Collection interval' })).toHaveValue(300);
    expect(screen.getByText('/var/lib/sensor-hub/sensor_hub.db')).toBeInTheDocument();
  });

  it('saves edits through PATCH /properties with database.path excluded from the payload', async () => {
    await renderPage(['view_properties', 'manage_properties']);

    fireEvent.change(screen.getByRole('spinbutton', { name: 'Collection interval' }), {
      target: { value: '120' },
    });
    fireEvent.click(screen.getByRole('button', { name: /save/i }));

    await waitFor(() => expect(patchMock).toHaveBeenCalledTimes(1));
    expect(patchMock).toHaveBeenCalledWith('/properties', {
      body: {
        'sensor.discovery.skip': 'true',
        'sensor.collection.interval': '120',
      },
    });
  });

  it('undoes one modified field back to the saved value, leaving other edits untouched', async () => {
    await renderPage(['view_properties', 'manage_properties']);

    fireEvent.change(screen.getByRole('spinbutton', { name: 'Collection interval' }), {
      target: { value: '120' },
    });
    fireEvent.click(screen.getByRole('switch', { name: 'Skip sensor discovery' }));

    fireEvent.click(screen.getByRole('button', { name: 'Undo changes to Collection interval' }));

    expect(screen.getByRole('spinbutton', { name: 'Collection interval' })).toHaveValue(300);
    expect(screen.getByRole('switch', { name: 'Skip sensor discovery' })).not.toBeChecked();
    expect(screen.queryByRole('button', { name: 'Undo changes to Collection interval' })).not.toBeInTheDocument();
  });

  it('treats a field typed back to the saved value as untouched, so a later server change reaches it', async () => {
    await renderPage(['view_properties', 'manage_properties']);

    const interval = screen.getByRole('spinbutton', { name: 'Collection interval' });
    fireEvent.change(interval, { target: { value: '120' } });
    fireEvent.change(interval, { target: { value: '300' } });

    act(() => {
      FakeWebSocket.instances[0].serverSends(
        JSON.stringify({ ...serverValues, 'sensor.collection.interval': '600' }),
      );
    });

    expect(screen.getByRole('spinbutton', { name: 'Collection interval' })).toHaveValue(600);
  });

  it('keeps an edited field on screen when a broadcast arrives, while untouched fields update', async () => {
    await renderPage(['view_properties', 'manage_properties']);

    fireEvent.change(screen.getByRole('spinbutton', { name: 'Collection interval' }), {
      target: { value: '120' },
    });

    act(() => {
      FakeWebSocket.instances[0].serverSends(
        JSON.stringify({ ...serverValues, 'sensor.discovery.skip': 'false' }),
      );
    });

    expect(screen.getByRole('spinbutton', { name: 'Collection interval' })).toHaveValue(120);
    expect(screen.getByRole('switch', { name: 'Skip sensor discovery' })).not.toBeChecked();
  });

  it('reports a broadcast landing on an edited field and resets it to the broadcast value', async () => {
    await renderPage(['view_properties', 'manage_properties']);

    fireEvent.change(screen.getByRole('spinbutton', { name: 'Collection interval' }), {
      target: { value: '120' },
    });

    act(() => {
      FakeWebSocket.instances[0].serverSends(
        JSON.stringify({ ...serverValues, 'sensor.collection.interval': '600' }),
      );
    });

    expect(screen.getByRole('spinbutton', { name: 'Collection interval' })).toHaveValue(120);
    expect(screen.getByText(/Someone else changed this to 600/)).toBeInTheDocument();
    expect(
      screen.queryByRole('button', { name: 'Undo changes to Collection interval' }),
    ).not.toBeInTheDocument();

    fireEvent.click(
      screen.getByRole('button', { name: 'Reset Collection interval to the value someone else saved' }),
    );

    expect(screen.getByRole('spinbutton', { name: 'Collection interval' })).toHaveValue(600);
    expect(screen.queryByText(/Someone else changed this/)).not.toBeInTheDocument();
    expect(screen.queryByText(/unsaved change/)).not.toBeInTheDocument();
  });

  it('states the unsaved count as plain text and offers Discard only while there are unsaved changes', async () => {
    await renderPage(['view_properties', 'manage_properties']);

    expect(screen.queryByText(/unsaved change/)).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Discard' })).not.toBeInTheDocument();

    fireEvent.change(screen.getByRole('spinbutton', { name: 'Collection interval' }), {
      target: { value: '120' },
    });
    expect(screen.getByText('1 unsaved change').closest('.MuiChip-root')).toBeNull();

    fireEvent.click(screen.getByRole('switch', { name: 'Skip sensor discovery' }));
    expect(screen.getByText('2 unsaved changes')).toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: 'Discard' }));

    expect(screen.getByRole('spinbutton', { name: 'Collection interval' })).toHaveValue(300);
    expect(screen.getByRole('switch', { name: 'Skip sensor discovery' })).toBeChecked();
    expect(screen.queryByText(/unsaved change/)).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Discard' })).not.toBeInTheDocument();
  });

  it('clears the edited fields on the broadcast that follows a successful save', async () => {
    await renderPage(['view_properties', 'manage_properties']);

    fireEvent.change(screen.getByRole('spinbutton', { name: 'Collection interval' }), {
      target: { value: '120' },
    });
    fireEvent.click(screen.getByRole('button', { name: /save/i }));
    await waitFor(() => expect(patchMock).toHaveBeenCalledTimes(1));

    act(() => {
      FakeWebSocket.instances[0].serverSends(
        JSON.stringify({ ...serverValues, 'sensor.collection.interval': '120' }),
      );
    });

    expect(screen.queryByText(/unsaved change/)).not.toBeInTheDocument();
    expect(
      screen.queryByRole('button', { name: 'Undo changes to Collection interval' }),
    ).not.toBeInTheDocument();

    act(() => {
      FakeWebSocket.instances[0].serverSends(
        JSON.stringify({ ...serverValues, 'sensor.collection.interval': '600' }),
      );
    });

    expect(screen.getByRole('spinbutton', { name: 'Collection interval' })).toHaveValue(600);
    expect(screen.queryByText(/unsaved change/)).not.toBeInTheDocument();
  });

  it('keeps a keystroke made while the save is still in flight', async () => {
    let settlePatch: (value: unknown) => void = () => {};
    patchMock.mockReturnValue(new Promise((resolve) => { settlePatch = resolve; }));
    await renderPage(['view_properties', 'manage_properties']);

    fireEvent.change(screen.getByRole('spinbutton', { name: 'Collection interval' }), {
      target: { value: '120' },
    });
    fireEvent.click(screen.getByRole('button', { name: /save/i }));
    await waitFor(() => expect(patchMock).toHaveBeenCalledTimes(1));

    fireEvent.change(screen.getByRole('spinbutton', { name: 'Collection interval' }), {
      target: { value: '121' },
    });
    await act(async () => { settlePatch({ data: { message: 'ok' } }); });

    act(() => {
      FakeWebSocket.instances[0].serverSends(
        JSON.stringify({ ...serverValues, 'sensor.collection.interval': '120' }),
      );
    });

    expect(screen.getByRole('spinbutton', { name: 'Collection interval' })).toHaveValue(121);
    expect(screen.getByText('1 unsaved change')).toBeInTheDocument();
  });

  it('disables every control and shows no save control for a user without manage_properties', async () => {
    await renderPage(['view_properties']);

    expect(screen.getByRole('switch', { name: 'Skip sensor discovery' })).toBeDisabled();
    expect(screen.getByRole('spinbutton', { name: 'Collection interval' })).toBeDisabled();
    expect(screen.queryByRole('button', { name: /save/i })).not.toBeInTheDocument();
  });

  it('renders one card per group, ordered by the definitions response, each owning its anchor id', async () => {
    await renderPage(['view_properties', 'manage_properties']);

    expect(screen.getAllByRole('heading', { level: 3 }).map((h) => h.textContent)).toEqual([
      'Sensors & collection',
      'Advanced',
    ]);

    const sensors = document.getElementById('sensors')!;
    expect(within(sensors).getByText('How often sensors are polled.')).toBeInTheDocument();
    expect(within(sensors).getByRole('switch', { name: 'Skip sensor discovery' })).toBeInTheDocument();
    expect(within(sensors).getByRole('spinbutton', { name: 'Collection interval' })).toBeInTheDocument();

    const advanced = document.getElementById('advanced')!;
    expect(within(advanced).getByText('/var/lib/sensor-hub/sensor_hub.db')).toBeInTheDocument();

    expect(sensors.compareDocumentPosition(advanced) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
  });

  it('scrolls to the group named by the URL fragment on load', async () => {
    const scrollIntoView = vi.fn();
    Element.prototype.scrollIntoView = scrollIntoView;
    window.location.hash = '#advanced';

    await renderPage(['view_properties', 'manage_properties']);

    expect(scrollIntoView.mock.instances[0]).toBe(document.getElementById('advanced'));
    delete (Element.prototype as Partial<Element>).scrollIntoView;
  });

  it('puts the search box in the rail above the group list', async () => {
    await renderPage(['view_properties', 'manage_properties']);

    const rail = screen.getByRole('navigation', { name: 'Property groups' });
    const search = within(rail).getByRole('textbox', { name: 'Search properties' });
    const firstGroup = screen.getByTestId('rail-sensors');

    expect(search.compareDocumentPosition(firstGroup) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
  });

  it('marks the group the page is scrolled to as current in the rail', async () => {
    await renderPage(['view_properties', 'manage_properties']);

    expect(screen.getByTestId('rail-sensors')).toHaveAttribute('aria-current', 'true');
    expect(screen.getByTestId('rail-advanced')).not.toHaveAttribute('aria-current');

    act(() => {
      FakeIntersectionObserver.instances[0].crossInto(['advanced']);
    });

    expect(screen.getByTestId('rail-advanced')).toHaveAttribute('aria-current', 'true');
    expect(screen.getByTestId('rail-sensors')).not.toHaveAttribute('aria-current');
  });

  it('shows only the rows matching the search term, across every group', async () => {
    await renderPage(['view_properties', 'manage_properties']);
    const search = screen.getByRole('textbox', { name: 'Search properties' });

    fireEvent.change(search, { target: { value: 'discovery.skip' } });
    expect(screen.getByText('Skip sensor discovery')).toBeInTheDocument();
    expect(screen.queryByText('Collection interval')).not.toBeInTheDocument();
    expect(screen.queryByText('Database file')).not.toBeInTheDocument();

    fireEvent.change(search, { target: { value: 'Skip sensor' } });
    expect(screen.getByText('Skip sensor discovery')).toBeInTheDocument();
    expect(screen.queryByText('Collection interval')).not.toBeInTheDocument();

    fireEvent.change(search, { target: { value: 'auto-discover' } });
    expect(screen.getByText('Skip sensor discovery')).toBeInTheDocument();
    expect(screen.queryByText('Collection interval')).not.toBeInTheDocument();

    fireEvent.change(search, { target: { value: 'SQLite' } });
    expect(screen.getByText('Database file')).toBeInTheDocument();
    expect(document.getElementById('sensors')).toBeNull();
    expect(screen.queryByTestId('rail-sensors')).not.toBeInTheDocument();
  });

  it('counts every edited field in a group, so the rail and the unsaved total agree under a filter', async () => {
    await renderPage(['view_properties', 'manage_properties']);

    expect(screen.queryByTestId('rail-edited-count-sensors')).not.toBeInTheDocument();
    expect(screen.queryByTestId('rail-edited-count-advanced')).not.toBeInTheDocument();

    fireEvent.change(screen.getByRole('spinbutton', { name: 'Collection interval' }), {
      target: { value: '120' },
    });
    fireEvent.click(screen.getByRole('switch', { name: 'Skip sensor discovery' }));

    expect(screen.getByTestId('rail-edited-count-sensors')).toHaveTextContent('2');
    expect(screen.queryByTestId('rail-edited-count-advanced')).not.toBeInTheDocument();

    fireEvent.change(screen.getByRole('textbox', { name: 'Search properties' }), {
      target: { value: 'auto-discover' },
    });

    expect(screen.getByTestId('rail-edited-count-sensors')).toHaveTextContent('2');
    expect(screen.getByText('2 unsaved changes')).toBeInTheDocument();
    expect(screen.queryByText('Collection interval')).not.toBeInTheDocument();
  });

  it('falls back to an editable text field per value, under a banner, when the definitions fetch fails', async () => {
    getMock.mockResolvedValue({ error: 'definitions unavailable' });
    await renderFallbackPage(['view_properties', 'manage_properties']);

    expect(
      screen.getByText(/descriptions and typed controls are unavailable/i),
    ).toBeInTheDocument();

    for (const key of Object.keys(serverValues)) {
      expect(screen.getByRole('textbox', { name: key })).toHaveValue(serverValues[key]);
    }
    expect(screen.queryByRole('switch')).not.toBeInTheDocument();
    expect(screen.queryByRole('spinbutton')).not.toBeInTheDocument();

    fireEvent.change(screen.getByRole('textbox', { name: 'sensor.collection.interval' }), {
      target: { value: '120' },
    });
    expect(screen.getByText('Saved value 300')).toBeInTheDocument();
    expect(screen.queryByText(/default/)).not.toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: /save/i }));

    await waitFor(() => expect(patchMock).toHaveBeenCalledTimes(1));
    expect(patchMock).toHaveBeenCalledWith('/properties', {
      body: { ...serverValues, 'sensor.collection.interval': '120' },
    });
  });

  it('puts a value with no matching definition in the Ungrouped section with its raw key', async () => {
    await renderPage(['view_properties', 'manage_properties']);

    act(() => {
      FakeWebSocket.instances[0].serverSends(
        JSON.stringify({ ...serverValues, 'orphan.property': 'kept' }),
      );
    });

    const ungrouped = document.getElementById('ungrouped')!;
    expect(within(ungrouped).getByRole('textbox', { name: 'orphan.property' })).toHaveValue('kept');
  });

  it('renders a definition with no matching value with its default as placeholder', async () => {
    await renderPage(['view_properties', 'manage_properties']);

    act(() => {
      FakeWebSocket.instances[0].serverSends(JSON.stringify({}));
    });

    const field = screen.getByRole('spinbutton', { name: 'Collection interval' });
    expect(field).toHaveValue(null);
    expect(field).toHaveAttribute('placeholder', '300');
    expect(screen.getByRole('switch', { name: 'Skip sensor discovery' })).not.toBeChecked();
  });

  it('says it is loading rather than rendering blank while the definitions are still in flight', async () => {
    let settleDefinitions: (value: unknown) => void = () => {};
    getMock.mockReturnValue(new Promise((resolve) => { settleDefinitions = resolve; }));

    const { default: PropertiesPage } = await import('./PropertiesPage');
    const { AuthContext } = await import('../providers/AuthContext');
    render(
      <AuthContext.Provider value={{ user: { id: 1, username: 'owner', roles: [], permissions: ['view_properties'] }, refresh: async () => {} }}>
        <PropertiesPage />
      </AuthContext.Provider>,
    );

    expect(screen.getByRole('heading', { name: 'Properties' })).toBeInTheDocument();
    expect(screen.getByText(/loading properties/i)).toBeInTheDocument();

    await act(async () => { settleDefinitions({ data: definitionsResponse }); });

    expect(screen.queryByText(/loading properties/i)).not.toBeInTheDocument();
    expect(screen.getByText('Skip sensor discovery')).toBeInTheDocument();
  });

  it('scrolls to the group named by the fragment even when the values arrive before the definitions', async () => {
    const scrollIntoView = vi.fn();
    Element.prototype.scrollIntoView = scrollIntoView;
    window.location.hash = '#advanced';

    let settleDefinitions: (value: unknown) => void = () => {};
    getMock.mockReturnValue(new Promise((resolve) => { settleDefinitions = resolve; }));

    const { default: PropertiesPage } = await import('./PropertiesPage');
    const { AuthContext } = await import('../providers/AuthContext');
    render(
      <AuthContext.Provider value={{ user: { id: 1, username: 'owner', roles: [], permissions: ['view_properties'] }, refresh: async () => {} }}>
        <PropertiesPage />
      </AuthContext.Provider>,
    );

    await waitFor(() => expect(FakeWebSocket.instances.length).toBe(1));
    act(() => {
      FakeWebSocket.instances[0].serverSends(JSON.stringify(serverValues));
    });
    expect(scrollIntoView).not.toHaveBeenCalled();

    await act(async () => { settleDefinitions({ data: definitionsResponse }); });

    expect(scrollIntoView.mock.instances[0]).toBe(document.getElementById('advanced'));
    delete (Element.prototype as Partial<Element>).scrollIntoView;
  });

  it('stacks the rail above the content at the mobile breakpoint', async () => {
    setViewportWidth(500);
    await renderPage(['view_properties', 'manage_properties']);

    expect(screen.getByTestId('properties-layout')).toHaveStyle({ flexDirection: 'column' });
  });

  it('sits the rail beside the content above the mobile breakpoint', async () => {
    setViewportWidth(1200);
    await renderPage(['view_properties', 'manage_properties']);

    expect(screen.getByTestId('properties-layout')).toHaveStyle({ flexDirection: 'row' });
  });
});
