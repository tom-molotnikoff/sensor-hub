import type { SensorHealthStatus } from '../gen/aliases';
import type { StatusKey } from '../ui/theme';

export const healthStatus: Record<SensorHealthStatus, StatusKey> = {
  good: 'ok',
  bad: 'bad',
  unknown: 'unknown',
};
