import SensorHealthHistoryChart from './SensorHealthHistoryChart';
import type { Sensor } from '../gen/aliases';
import Card from '../ui/Card';

interface SensorHealthHistoryChartCardProps {
  sensor: Sensor;
}

export default function SensorHealthHistoryChartCard({ sensor }: SensorHealthHistoryChartCardProps) {
  return (
    <Card title="Sensor Health History">
      <SensorHealthHistoryChart sensor={sensor} />
    </Card>
  );
}
