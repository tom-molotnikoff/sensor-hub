import { useRef, useState } from 'react';
import type { CommandStatusMessage, Sensor } from '../gen/aliases';
import { apiClient } from '../gen/client';
import { requestScheduler } from '../scheduler/requestScheduler';
import { useCurrentReadings, type CurrentReadingsMap } from '../hooks/useCurrentReadings';

// 'unsent' is a command the hub refused, so it has no status to wait for.
export type CommandPhase = 'idle' | 'pending' | 'acknowledged' | 'failed' | 'timed_out' | 'unsent';

interface PendingCommand {
  id: number;
  onRejected?: () => void;
}

interface SensorCommandOptions {
  sensor: Sensor | undefined;
  property: string | undefined;
  onDataUpdate?: (at: Date) => void;
}

export interface SensorCommand {
  readings: CurrentReadingsMap;
  phase: CommandPhase;
  notice: string | null;
  dismissNotice: () => void;
  // onRejected runs when the hub refuses the command or the command fails or
  // times out, so the caller can undo what it showed optimistically.
  send: (value: string, onRejected?: () => void) => Promise<void>;
}

// Sends a property command for a dashboard control and follows it to its
// acknowledgement over the current-readings socket.
export function useSensorCommand({ sensor, property, onDataUpdate }: SensorCommandOptions): SensorCommand {
  const pendingRef = useRef<PendingCommand | null>(null);
  const [phase, setPhase] = useState<CommandPhase>('idle');
  const [notice, setNotice] = useState<string | null>(null);

  // useCurrentReadings keeps callbacks in refs, so this needs no memoization.
  const handleCommandStatus = (message: CommandStatusMessage) => {
    const pending = pendingRef.current;
    if (!pending || !sensor || !property) return;
    if (message.id !== pending.id || message.sensor_id !== sensor.id || message.property !== property) return;
    pendingRef.current = null;

    if (message.status === 'failed' || message.status === 'timed_out') {
      pending.onRejected?.();
      setPhase(message.status);
      setNotice(message.status === 'timed_out' ? 'Command timed out' : 'Command failed');
      onDataUpdate?.(new Date());
      return;
    }
    if (message.status === 'acknowledged') {
      setPhase('acknowledged');
    }
  };

  const readings = useCurrentReadings({ onDataUpdate, onCommandStatus: handleCommandStatus });

  const send = async (value: string, onRejected?: () => void) => {
    if (!sensor || !property) return;
    setPhase('pending');

    // Pause low-priority background polls while the command is sent, so it
    // and its confirmation aren't queued behind the read-only chart flood.
    const { data, error } = await requestScheduler.runWithPreemption(() => apiClient.POST('/sensors/{id}/command', {
      params: { path: { id: sensor.id } },
      body: { property, value },
    }));

    if (error) {
      onRejected?.();
      setPhase('unsent');
      setNotice('Failed to send command');
      return;
    }

    if (data) {
      pendingRef.current = { id: data.id, onRejected };
    }
  };

  return { readings, phase, notice, dismissNotice: () => setNotice(null), send };
}
