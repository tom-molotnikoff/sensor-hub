import useTotalReadingsForEachSensor from "../hooks/useTotalReadingsForEachSensor.ts";
import BarChartOutlinedIcon from "@mui/icons-material/BarChartOutlined";
import { Typography } from "@mui/material";
import Card from "../ui/Card";
import DataTable from "../ui/DataTable";
import EmptyState from '../ui/EmptyState';
import { WidgetSwap, CascadeRowsLoader } from "../ui/loaders";

function formatSampledAt(sampledAt: string): string {
  const parsed = new Date(sampledAt);
  return Number.isNaN(parsed.getTime()) ? '' : parsed.toLocaleString();
}

interface TotalReadingsForEachSensorCardProps {
  pollIntervalMs?: number;
}

function TotalReadingsForEachSensorCard({ pollIntervalMs }: TotalReadingsForEachSensorCardProps) {
  const [sample, isLoading] = useTotalReadingsForEachSensor(pollIntervalMs);

  const rows = Object.entries(sample.counts).map(([sensor, totalReadings], index) => ({
    id: index,
    sensor,
    totalReadings,
  }));

  const sampledAt = rows.length > 0 ? formatSampledAt(sample.sampled_at) : '';

  return (
    <Card
      title="Total Readings For Each Sensor"
      actions={sampledAt && (
        <Typography variant="caption" sx={{ color: "text.secondary" }}>
          Sampled {sampledAt}
        </Typography>
      )}
    >
      <WidgetSwap loading={isLoading} loader={<CascadeRowsLoader />}>
        {rows.length === 0 ? (
          <EmptyState
            icon={<BarChartOutlinedIcon fontSize="large" />}
            title="No reading data yet"
            description="Readings will appear here once sensors start collecting data."
          />
        ) : (
          <DataTable
            rows={rows}
            columns={[
              { field: 'sensor', headerName: 'Sensor', flex: 1, compact: 'title' },
              { field: 'totalReadings', headerName: 'Total Readings', type: 'number', flex: 1, compact: 'meta' },
            ]}
          />
        )}
      </WidgetSwap>
    </Card>
  );
}

export default TotalReadingsForEachSensorCard;
