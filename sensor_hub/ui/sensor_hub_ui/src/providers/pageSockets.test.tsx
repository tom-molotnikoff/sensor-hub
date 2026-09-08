import { act, render } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { useCurrentReadings } from '../hooks/useCurrentReadings';
import { useProperties } from '../hooks/useProperties';
import { FakeWebSocket, installFakeWebSocket } from '../test/fakeWebSocket';
import { AuthContext } from './AuthContext';
import CurrentReadingsProvider from './CurrentReadingsProvider';
import PropertiesProvider from './PropertiesProvider';

const WIDGETS_ON_THE_PAGE = 12;

const user = { id: 1, username: 'operator', roles: [], permissions: ['view_readings'] };

function WidgetFrame() {
  const properties = useProperties();
  const readings = useCurrentReadings();
  return (
    <div data-testid="widget-frame">
      {properties['weather.location.name'] ?? 'no location'}
      {' / '}
      {readings['office-plug']?.state?.text_state ?? 'no reading'}
    </div>
  );
}

function Page() {
  return (
    <AuthContext.Provider value={{ user, refresh: async () => {} }}>
      <PropertiesProvider>
        <CurrentReadingsProvider>
          {Array.from({ length: WIDGETS_ON_THE_PAGE }, (_, index) => <WidgetFrame key={index} />)}
        </CurrentReadingsProvider>
      </PropertiesProvider>
    </AuthContext.Provider>
  );
}

function socketsFor(path: string): FakeWebSocket[] {
  return FakeWebSocket.instances.filter((socket) => socket.url.endsWith(path));
}

describe('page sockets', () => {
  let restoreWebSocket: () => void;

  beforeEach(() => {
    restoreWebSocket = installFakeWebSocket();
  });

  afterEach(() => {
    restoreWebSocket();
    vi.useRealTimers();
  });

  it('opens one properties socket and one current-readings socket for the whole page', () => {
    const { getAllByTestId } = render(<Page />);

    expect(getAllByTestId('widget-frame')).toHaveLength(WIDGETS_ON_THE_PAGE);
    expect(socketsFor('/properties/ws')).toHaveLength(1);
    expect(socketsFor('/readings/ws/current')).toHaveLength(1);
  });

  it('reconnects the properties socket on the same backoff as the current-readings socket', () => {
    vi.useFakeTimers();
    render(<Page />);

    act(() => {
      socketsFor('/properties/ws')[0].serverCloses();
      socketsFor('/readings/ws/current')[0].serverCloses();
    });

    act(() => { vi.advanceTimersByTime(999); });
    expect(socketsFor('/properties/ws')).toHaveLength(1);
    expect(socketsFor('/readings/ws/current')).toHaveLength(1);

    act(() => { vi.advanceTimersByTime(1); });
    expect(socketsFor('/properties/ws')).toHaveLength(2);
    expect(socketsFor('/readings/ws/current')).toHaveLength(2);

    act(() => {
      socketsFor('/properties/ws')[1].serverCloses();
      socketsFor('/readings/ws/current')[1].serverCloses();
    });

    act(() => { vi.advanceTimersByTime(1999); });
    expect(socketsFor('/properties/ws')).toHaveLength(2);
    expect(socketsFor('/readings/ws/current')).toHaveLength(2);

    act(() => { vi.advanceTimersByTime(1); });
    expect(socketsFor('/properties/ws')).toHaveLength(3);
    expect(socketsFor('/readings/ws/current')).toHaveLength(3);
  });

  it('feeds every widget frame from the one socket of each kind', () => {
    const { getAllByTestId } = render(<Page />);

    act(() => {
      socketsFor('/properties/ws')[0].serverSends(JSON.stringify({ 'weather.location.name': 'Leeds' }));
      socketsFor('/readings/ws/current')[0].serverSends(JSON.stringify([{
        id: 99,
        sensor_name: 'office-plug',
        measurement_type: 'state',
        numeric_value: null,
        text_state: 'ON',
        unit: '',
        time: '2026-05-09T20:00:00Z',
      }]));
    });

    const frames = getAllByTestId('widget-frame');
    expect(frames).toHaveLength(WIDGETS_ON_THE_PAGE);
    frames.forEach((frame) => expect(frame).toHaveTextContent('Leeds / ON'));
  });
});
