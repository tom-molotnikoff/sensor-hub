import { render, screen, waitFor } from '@testing-library/react';
import { MemoryRouter } from 'react-router';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import type { TotalReadingsSample } from '../gen/aliases';
import TotalReadingsForEachSensorCard from './TotalReadingsForEachSensorCard';

const { getMock } = vi.hoisted(() => ({
  getMock: vi.fn(),
}));

vi.mock('../gen/client', () => ({
  apiClient: {
    GET: getMock,
  },
}));

vi.mock('../providers/AuthContext', () => ({
  useAuth: () => ({ user: { id: 1, username: 'operator', roles: [], permissions: [] } }),
}));

vi.mock('../hooks/useSensorContext', () => ({
  useSensorContext: () => ({ loaded: true }),
}));

vi.mock('../tools/logger', () => ({
  logger: { error: vi.fn() },
}));

function respondWith(sample: TotalReadingsSample) {
  getMock.mockResolvedValue({ data: sample });
}

function renderCard() {
  render(
    <MemoryRouter>
      <TotalReadingsForEachSensorCard />
    </MemoryRouter>,
  );
}

describe('TotalReadingsForEachSensorCard', () => {
  beforeEach(() => {
    getMock.mockReset();
  });

  it('shows the sampled time next to the counts', async () => {
    const sampledAt = '2026-09-08T10:30:00Z';
    respondWith({ sampled_at: sampledAt, counts: { Office: 120 } });

    renderCard();

    await waitFor(() => {
      expect(screen.getByText('Office')).toBeInTheDocument();
      expect(screen.getByText((120).toLocaleString())).toBeInTheDocument();
      expect(screen.getByText(`Sampled ${new Date(sampledAt).toLocaleString()}`)).toBeInTheDocument();
    });
  });

  it('shows no sampled time before any counts arrive', async () => {
    respondWith({ sampled_at: '0001-01-01T00:00:00Z', counts: {} });

    renderCard();

    expect(await screen.findByText('No reading data yet')).toBeInTheDocument();
    expect(screen.queryByText(/^Sampled /)).not.toBeInTheDocument();
  });
});
