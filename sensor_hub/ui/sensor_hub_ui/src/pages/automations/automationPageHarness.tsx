import { ThemeProvider } from '@mui/material';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { act, render, waitFor } from '@testing-library/react';
import type { ReactElement, ReactNode } from 'react';
import { MemoryRouter, Route, Routes } from 'react-router';
import { vi, type Mock } from 'vitest';
import type { MeResponse, Sensor } from '../../gen/aliases';
import RequireAuth from '../../navigation/RequireAuth';
import AuthProvider from '../../providers/AuthProvider';
import { SensorContextProvider } from '../../providers/SensorContext';
import { FakeWebSocket } from '../../test/fakeWebSocket';
import { theme } from '../../ui/theme';

export const editorPermissions = ['view_automations', 'manage_automations', 'control_sensors'];

function signedIn(permissions: string[]): MeResponse {
  return {
    user: {
      id: 1,
      username: 'tom',
      email: 'tom@example.com',
      disabled: false,
      must_change_password: false,
      roles: [],
      permissions,
      created_at: '2026-10-01T00:00:00Z',
      updated_at: '2026-10-01T00:00:00Z',
    },
  };
}

export function serveGets(get: Mock, permissions: string[], responses: Record<string, unknown>) {
  get.mockImplementation(async (path: string) => {
    const data = path === '/auth/me' ? signedIn(permissions) : responses[path];
    return data === undefined
      ? { error: { message: `nothing served for ${path}` }, response: new Response(null, { status: 404 }) }
      : { data, response: new Response() };
  });
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

interface RenderOptions {
  routes: Record<string, ReactElement>;
  at: string;
  width: number;
  sensors: Sensor[];
  alongside?: ReactNode;
}

export async function renderAutomationPages({ routes, at, width, sensors, alongside }: RenderOptions) {
  atWidth(width);
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  const rendered = render(
    <ThemeProvider theme={theme}>
      <QueryClientProvider client={client}>
        <AuthProvider>
          <SensorContextProvider>
            <MemoryRouter initialEntries={[at]}>
              <Routes>
                {Object.entries(routes).map(([path, element]) => (
                  <Route key={path} path={path} element={<RequireAuth permission="view_automations">{element}</RequireAuth>} />
                ))}
              </Routes>
              {alongside}
            </MemoryRouter>
          </SensorContextProvider>
        </AuthProvider>
      </QueryClientProvider>
    </ThemeProvider>,
  );
  const socket = await waitFor(() => {
    const opened = FakeWebSocket.instances.find((each) => each.url.endsWith('/sensors/ws'));
    if (!opened) throw new Error('the sensors socket is not open yet');
    return opened;
  });
  act(() => socket.serverSends(JSON.stringify(sensors)));
  return rendered;
}
