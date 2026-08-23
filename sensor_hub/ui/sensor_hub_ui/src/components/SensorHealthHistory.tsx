import type {Sensor} from "../gen/aliases";
import useSensorHealthHistory from "../hooks/useSensorHealthHistory.ts";
import {useState} from "react";
import {DataGrid, type GridColDef} from "@mui/x-data-grid";
import LayoutCard from "../tools/LayoutCard.tsx";
import {TypographyH2} from "../tools/Typography.tsx";
import {Alert, Button, Snackbar} from "@mui/material";
import RefreshIcon from '@mui/icons-material/Refresh';
import { useIsMobile } from "../hooks/useMobile";

interface SensorHealthHistoryProps {
  sensor: Sensor,
}

function SensorHealthHistory({sensor}: SensorHealthHistoryProps) {
  const [healthHistory, refresh, isLoading] = useSensorHealthHistory(sensor.name);
  const [snackbarOpen, setSnackbarOpen] = useState(false);
  const isMobile = useIsMobile();

  const rows = healthHistory.map((entry) => ({
    id: entry.id,
    health_status: entry.health_status,
    recorded_at: new Date(entry.recorded_at).toLocaleString(),
  }));

  const columns: GridColDef[] = [
    { field: "health_status", headerName: "Health Status", flex: 1, minWidth: 150 },
    { field: "recorded_at", headerName: "Recorded At", flex: 1, minWidth: 200 },
  ]


  return (
    <LayoutCard variant="secondary" changes={{width: "100%", minHeight: 400}}>
      <TypographyH2>Sensor Health History</TypographyH2>
      <div
        style={{
          minHeight: 450,
          display: "flex",
          flexDirection: "column",
          width: "100%",
        }}
      >
        <DataGrid
          showToolbar
          rows={rows}
          columns={columns}
          initialState={{
            pagination: {
              paginationModel: { pageSize: 5, page: 0 },
            },
          }}
          loading={isLoading}
          sx={{
            backgroundColor: 'background.paper',
            borderRadius: 2,
            mt: 2,
            '& .MuiDataGrid-columnHeaders': { fontWeight: 'bold' },
          }}
        />
        <div style={{
          display: "flex",
          flexDirection: isMobile ? "column" : "row",
          justifyContent: isMobile ? "center" : "flex-end",
          alignItems: isMobile ? "stretch" : "center",
          flexGrow: 1,
          width: "100%",
          marginTop: 16,
          gap: 16
        }}>
          <Button
            onClick={() => {
              refresh().then(() => {
                setSnackbarOpen(true);
              });
            }}
            variant="outlined"
            startIcon={<RefreshIcon />}
            fullWidth={isMobile}
            sx={{
              mt: 2,
              alignSelf: 'center',
              height: "56px",
              width: isMobile ? "100%" : undefined,
            }}
          >
            Refresh
          </Button>
        </div>


      </div>
      <Snackbar
        open={snackbarOpen}
        onClose={() => setSnackbarOpen(false)}
        autoHideDuration={2000}
        anchorOrigin={{ vertical: 'bottom', horizontal: 'center' }}
      >
        <Alert sx={{ width: '100%' }}>
          Sensor health history refreshed.
        </Alert>
      </Snackbar>
    </LayoutCard>
  );
}

export default SensorHealthHistory;
