import type { Sensor } from "../gen/aliases";
import useSensorHealthHistory from "../hooks/useSensorHealthHistory.ts";
import { useState } from "react";
import { Alert, Button, Snackbar } from "@mui/material";
import RefreshIcon from '@mui/icons-material/Refresh';
import { healthStatus } from "../tools/healthStatus";
import Card from "../ui/Card";
import DataTable from "../ui/DataTable";
import Inline from "../ui/Inline";
import Stack from "../ui/Stack";

interface SensorHealthHistoryProps {
  sensor: Sensor,
}

function SensorHealthHistory({ sensor }: SensorHealthHistoryProps) {
  const [healthHistory, refresh, isLoading] = useSensorHealthHistory(sensor.name);
  const [snackbarOpen, setSnackbarOpen] = useState(false);

  const rows = healthHistory.map((entry) => ({
    id: entry.id,
    health_status: entry.health_status,
    recorded_at: new Date(entry.recorded_at).toLocaleString(),
  }));

  return (
    <Card title="Sensor Health History">
      <Stack>
        <DataTable
          rows={rows}
          loading={isLoading}
          columns={[
            {
              field: "health_status",
              headerName: "Health Status",
              flex: 1,
              minWidth: 150,
              compact: 'status',
              statusOf: (row) => healthStatus[row.health_status],
            },
            { field: "recorded_at", headerName: "Recorded At", flex: 1, minWidth: 200, compact: 'title' },
          ]}
        />
        <Inline>
          <Button
            onClick={() => {
              refresh().then(() => {
                setSnackbarOpen(true);
              });
            }}
            variant="outlined"
            startIcon={<RefreshIcon />}
          >
            Refresh
          </Button>
        </Inline>
      </Stack>
      <Snackbar
        open={snackbarOpen}
        onClose={() => setSnackbarOpen(false)}
        autoHideDuration={2000}
        anchorOrigin={{ vertical: 'bottom', horizontal: 'center' }}
      >
        <Alert>
          Sensor health history refreshed.
        </Alert>
      </Snackbar>
    </Card>
  );
}

export default SensorHealthHistory;
