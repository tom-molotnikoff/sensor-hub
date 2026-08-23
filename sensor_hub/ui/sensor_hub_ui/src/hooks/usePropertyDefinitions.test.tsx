import { renderHook, waitFor } from '@testing-library/react';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import type { PropertyDefinitionsResponse } from '../gen/aliases';

const { getMock } = vi.hoisted(() => ({
  getMock: vi.fn(),
}));

vi.mock('../gen/client', () => ({
  apiClient: {
    GET: getMock,
  },
}));

const response: PropertyDefinitionsResponse = {
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
  ],
  groups: [
    { id: 'sensors', label: 'Sensors & collection', description: 'How often sensors are polled.', order: 1 },
  ],
};

describe('usePropertyDefinitions', () => {
  beforeEach(() => {
    getMock.mockReset();
    vi.resetModules();
  });

  it('exposes the definitions fetched from the endpoint', async () => {
    getMock.mockResolvedValue({ data: response });
    const { usePropertyDefinitions } = await import('./usePropertyDefinitions');

    const { result } = renderHook(() => usePropertyDefinitions());

    await waitFor(() => expect(result.current.loading).toBe(false));
    expect(result.current.definitions?.definitions[0]?.key).toBe('sensor.discovery.skip');
    expect(result.current.error).toBeNull();
  });

  it('fetches the definitions once for the session, not once per mount', async () => {
    getMock.mockResolvedValue({ data: response });
    const { usePropertyDefinitions } = await import('./usePropertyDefinitions');

    const first = renderHook(() => usePropertyDefinitions());
    await waitFor(() => expect(first.result.current.loading).toBe(false));
    first.unmount();

    const second = renderHook(() => usePropertyDefinitions());
    await waitFor(() => expect(second.result.current.loading).toBe(false));

    expect(second.result.current.definitions?.definitions[0]?.key).toBe('sensor.discovery.skip');
    expect(getMock).toHaveBeenCalledTimes(1);
  });
});
