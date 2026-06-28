import useTotalReadingsForEachSensor from "../hooks/useTotalReadingsForEachSensor.ts";
import {DataGrid, type GridColDef} from "@mui/x-data-grid";
import {TypographyH2} from "../tools/Typography.tsx";
import LayoutCard from "../tools/LayoutCard.tsx";
import RefreshIcon from "@mui/icons-material/Refresh";
import BarChartOutlinedIcon from "@mui/icons-material/BarChartOutlined";
import {Alert, Button, Snackbar} from "@mui/material";
import {useState} from "react";
import { useSensorContext } from "../hooks/useSensorContext.ts";
import EmptyState from "./EmptyState.tsx";
import { CascadeRowsLoader } from "../dashboard/widget-loaders";

function TotalReadingsForEachSensorCard({ showTitle = true }: { showTitle?: boolean }) {
  const [data, refresh] = useTotalReadingsForEachSensor();
  const [snackbarOpen, setSnackbarOpen] = useState(false);
  const [refreshing, setRefreshing] = useState(false);

  const { loaded } = useSensorContext();

  const columns: GridColDef[] = [
    { field: 'sensor', headerName: 'Sensor', flex: 1 },
    { field: 'totalReadings', headerName: 'Total Readings', type: 'number', flex: 1 },
  ];

  const rows = Object.entries(data).map(([sensor, totalReadings], index) => ({
    id: index,
    sensor,
    totalReadings,
  }));

  return (
    <LayoutCard variant="secondary" changes={{height: "100%", width: "100%"}}>
      <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', width: '100%' }}>
        {showTitle && <TypographyH2>Total Readings For Each Sensor</TypographyH2>}
        <Button
          onClick={() => {
            setRefreshing(true);
            refresh().then(() => {
              setRefreshing(false);
              setSnackbarOpen(true);
            });
          }}
          variant="outlined" startIcon={<RefreshIcon />}
          disabled={refreshing}
          size="small"
        >
          Refresh
        </Button>
      </div>

      <div style={{ flex: 1, minHeight: 0, width: '100%' }}>
        {!loaded ? (
          <CascadeRowsLoader />
        ) : rows.length === 0 ? (
          <EmptyState
            icon={<BarChartOutlinedIcon sx={{ fontSize: 48 }} />}
            title="No reading data yet"
            description="Readings will appear here once sensors start collecting data."
          />
        ) : (
          <DataGrid
            showToolbar
            rows={rows}
            columns={columns}
            pageSizeOptions={[5, 10, 25, 50, 100]}
            initialState={{
              pagination: {
                paginationModel: { pageSize: 5, page: 0 },
              },
            }}
            sx={{
              height: '100%',
              backgroundColor: 'background.paper',
              borderRadius: 2,
              '& .MuiDataGrid-columnHeaders': { fontWeight: 'bold' },
            }}
          />
        )}
      </div>
      <Snackbar
        open={snackbarOpen}
        onClose={() => setSnackbarOpen(false)}
        autoHideDuration={2000}
        anchorOrigin={{ vertical: 'bottom', horizontal: 'center' }}
      >
        <Alert sx={{ width: '100%' }}>
          Total Readings Per Sensor refreshed.
        </Alert>
      </Snackbar>
    </LayoutCard>
  );
}

export default TotalReadingsForEachSensorCard;