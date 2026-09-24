import { useEffect, useState, useCallback } from 'react';
import { Chip, Typography } from '@mui/material';
import { apiClient } from '../gen/client';
import type { MQTTBrokerStats } from '../gen/aliases';
import Card from '../ui/Card';
import Inline from '../ui/Inline';
import PageGrid from '../ui/PageGrid';
import Stack from '../ui/Stack';
import { logger } from '../tools/logger';
import WifiIcon from '@mui/icons-material/Wifi';
import WifiOffIcon from '@mui/icons-material/WifiOff';
import MessageIcon from '@mui/icons-material/Message';
import ErrorOutlineIcon from '@mui/icons-material/ErrorOutlined';
import DevicesIcon from '@mui/icons-material/Devices';

function formatRelativeTime(iso: string | null): string {
  if (!iso) return 'Never';
  const date = new Date(iso);
  const now = new Date();
  const diffMs = now.getTime() - date.getTime();
  if (diffMs < 0) return 'Just now';
  const secs = Math.floor(diffMs / 1000);
  if (secs < 60) return `${secs}s ago`;
  const mins = Math.floor(secs / 60);
  if (mins < 60) return `${mins}m ago`;
  const hrs = Math.floor(mins / 60);
  if (hrs < 24) return `${hrs}h ago`;
  return `${Math.floor(hrs / 24)}d ago`;
}

function formatUptime(iso: string | null): string {
  if (!iso) return '—';
  const date = new Date(iso);
  const now = new Date();
  const diffMs = now.getTime() - date.getTime();
  if (diffMs < 0) return 'Just connected';
  const secs = Math.floor(diffMs / 1000);
  if (secs < 60) return `${secs}s`;
  const mins = Math.floor(secs / 60);
  if (mins < 60) return `${mins}m ${secs % 60}s`;
  const hrs = Math.floor(mins / 60);
  if (hrs < 24) return `${hrs}h ${mins % 60}m`;
  return `${Math.floor(hrs / 24)}d ${hrs % 24}h`;
}

function BrokerStatCard({ stat }: { stat: MQTTBrokerStats }) {
  const totalErrors = stat.parse_errors + stat.processing_errors;
  return (
    <Card
      variant="inset"
      title={stat.broker_name || `Broker ${stat.broker_id}`}
      actions={
        <>
          {stat.connected
            ? <WifiIcon color="success" fontSize="small" />
            : <WifiOffIcon color="error" fontSize="small" />}
          <Chip
            size="small"
            label={stat.connected ? 'Connected' : 'Disconnected'}
            color={stat.connected ? 'success' : 'error'}
            variant="outlined"
          />
        </>
      }
    >
      <Stack>
        <Inline>
          <MessageIcon fontSize="small" sx={{ color: 'text.secondary' }} />
          <Typography variant="body2" sx={{ color: "text.secondary" }}>
            Messages: <strong>{stat.messages_received.toLocaleString()}</strong>
          </Typography>
        </Inline>

        {totalErrors > 0 && (
          <Inline>
            <ErrorOutlineIcon fontSize="small" sx={{ color: 'warning.main' }} />
            <Typography variant="body2" sx={{ color: "warning.main" }}>
              Errors: <strong>{totalErrors}</strong>
              {stat.parse_errors > 0 && ` (${stat.parse_errors} parse)`}
              {stat.processing_errors > 0 && ` (${stat.processing_errors} processing)`}
            </Typography>
          </Inline>
        )}

        {stat.devices_discovered > 0 && (
          <Inline>
            <DevicesIcon fontSize="small" sx={{ color: 'text.secondary' }} />
            <Typography variant="body2" sx={{ color: "text.secondary" }}>
              Devices discovered: <strong>{stat.devices_discovered}</strong>
            </Typography>
          </Inline>
        )}

        <Typography variant="body2" sx={{ color: "text.secondary" }}>
          Last message: {formatRelativeTime(stat.last_message_at ?? null)}
        </Typography>

        {stat.connected && (
          <Typography variant="body2" sx={{ color: "text.secondary" }}>
            Uptime: {formatUptime(stat.connected_since ?? null)}
          </Typography>
        )}
      </Stack>
    </Card>
  );
}

export default function MqttStatsCard() {
  const [stats, setStats] = useState<MQTTBrokerStats[]>([]);
  const [, setTick] = useState(0);

  const load = useCallback(() =>
    apiClient.GET('/mqtt/stats')
      .then(({ data: s }) => setStats((s as MQTTBrokerStats[] | null) ?? []))
      .catch((e) => logger.error(e)),
  []);

  useEffect(() => {
    void load();
    const interval = setInterval(() => {
      load();
      setTick(t => t + 1);
    }, 10_000);
    return () => clearInterval(interval);
  }, [load]);

  return (
    <Card title="MQTT Broker Stats">
      {stats.length === 0 ? (
        <Typography variant="body2" sx={{ color: "text.secondary" }}>
          No MQTT broker statistics available. Connect a broker to see live stats.
        </Typography>
      ) : (
        <PageGrid>
          {stats.map(s => (
            <PageGrid.Item key={s.broker_id} span={{ wide: 4 }}>
              <BrokerStatCard stat={s} />
            </PageGrid.Item>
          ))}
        </PageGrid>
      )}
    </Card>
  );
}
