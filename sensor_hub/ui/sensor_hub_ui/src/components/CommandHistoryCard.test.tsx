import { ThemeProvider } from '@mui/material';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { render, screen, within } from '@testing-library/react';
import { MemoryRouter } from 'react-router';
import { afterEach, describe, expect, it, vi } from 'vitest';
import type { CommandHistoryEntry } from '../gen/aliases';
import { theme } from '../ui/theme';
import CommandHistoryCard from './CommandHistoryCard';

const api = vi.hoisted(() => ({ GET: vi.fn() }));

vi.mock('../gen/client', () => ({ apiClient: api }));

function command(id: number, overrides: Partial<CommandHistoryEntry>): CommandHistoryEntry {
  return {
    id,
    property: 'state',
    value: 'ON',
    status: 'acknowledged',
    sent_at: '2026-10-06T18:00:00Z',
    timeout_seconds: 10,
    mqtt_topic: 'zigbee2mqtt/office-plug/set',
    mqtt_payload: '{"state":"ON"}',
    ...overrides,
  };
}

function atWidth(width: number) {
  vi.stubGlobal('matchMedia', (query: string) => ({
    matches: Number(/\(min-width:\s*(\d+)px\)/.exec(query)?.[1] ?? 0) <= width,
    media: query,
    addListener: () => {},
    removeListener: () => {},
    addEventListener: () => {},
    removeEventListener: () => {},
  }));
}

describe('CommandHistoryCard', () => {
  afterEach(() => vi.unstubAllGlobals());

  it("names the automation that sent a command, linking to its editor, and the user for anyone else's", async () => {
    atWidth(1280);
    api.GET.mockResolvedValue({
      data: [
        command(2, { automation_run_id: 7, automation: { id: 3, name: 'Evening lights' } }),
        command(1, { value: 'OFF', user: { id: 1, username: 'tom' } }),
      ],
      response: new Response(),
    });
    render(
      <ThemeProvider theme={theme}>
        <QueryClientProvider client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}>
          <MemoryRouter>
            <CommandHistoryCard sensorId={6} />
          </MemoryRouter>
        </QueryClientProvider>
      </ThemeProvider>,
    );

    expect(await screen.findByRole('link', { name: 'Evening lights' })).toHaveAttribute('href', '/automations/3');
    const byUser = (await screen.findByText('state = OFF')).closest<HTMLElement>('[role=row]')!;
    expect(within(byUser).getByText('tom')).toBeInTheDocument();
    expect(within(byUser).queryByRole('link')).toBeNull();
  });
});
