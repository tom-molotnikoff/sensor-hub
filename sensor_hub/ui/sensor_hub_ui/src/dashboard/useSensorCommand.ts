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

// The hub answers the POST after publishing, so a quick device can be
// acknowledged before the command's id is known. Statuses for this control that
// arrive while a POST is outstanding are kept, up to this many, and the one for
// the returned id is applied once the POST returns.
const MAX_EARLY_STATUSES = 16;

// Sends a property command for a dashboard control and follows it to its
// acknowledgement over the current-readings socket.
export function useSensorCommand({ sensor, property, onDataUpdate }: SensorCommandOptions): SensorCommand {
  const pendingRef = useRef<PendingCommand | null>(null);
  const sendingRef = useRef(false);
  const earlyStatusesRef = useRef(new Map<number, CommandStatusMessage>());
  const [phase, setPhase] = useState<CommandPhase>('idle');
  const [notice, setNotice] = useState<string | null>(null);

  const settle = (message: CommandStatusMessage, onRejected?: () => void) => {
    if (message.status === 'failed' || message.status === 'timed_out') {
      onRejected?.();
      setPhase(message.status);
      setNotice(message.status === 'timed_out' ? 'Command timed out' : 'Command failed');
      onDataUpdate?.(new Date());
      return;
    }
    if (message.status === 'acknowledged') {
      setPhase('acknowledged');
    }
  };

  // useCurrentReadings keeps callbacks in refs, so this needs no memoization.
  const handleCommandStatus = (message: CommandStatusMessage) => {
    if (!sensor || !property || message.sensor_id !== sensor.id || message.property !== property) return;
    const pending = pendingRef.current;
    if (pending && message.id === pending.id) {
      pendingRef.current = null;
      settle(message, pending.onRejected);
      return;
    }
    if (sendingRef.current) {
      const early = earlyStatusesRef.current;
      early.set(message.id, message);
      if (early.size > MAX_EARLY_STATUSES) {
        early.delete(early.keys().next().value as number);
      }
    }
  };

  const readings = useCurrentReadings({ onDataUpdate, onCommandStatus: handleCommandStatus });

  const send = async (value: string, onRejected?: () => void) => {
    if (!sensor || !property) return;
    setPhase('pending');
    sendingRef.current = true;
    earlyStatusesRef.current.clear();

    // Pause low-priority background polls while the command is sent, so it
    // and its confirmation aren't queued behind the read-only chart flood.
    const { data, error } = await requestScheduler.runWithPreemption(() => apiClient.POST('/sensors/{id}/command', {
      params: { path: { id: sensor.id } },
      body: { property, value },
    }));
    sendingRef.current = false;
    const early = data ? earlyStatusesRef.current.get(data.id) : undefined;
    earlyStatusesRef.current.clear();

    if (error) {
      onRejected?.();
      setPhase('unsent');
      setNotice('Failed to send command');
      return;
    }

    if (early) {
      settle(early, onRejected);
    } else if (data) {
      pendingRef.current = { id: data.id, onRejected };
    }
  };

  return { readings, phase, notice, dismissNotice: () => setNotice(null), send };
}
