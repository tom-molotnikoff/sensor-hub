import useTotalReadingsForEachSensor from "../hooks/useTotalReadingsForEachSensor.ts";
import {DataGrid, type GridColDef} from "@mui/x-data-grid";
import {TypographyH2} from "../tools/Typography.tsx";
import LayoutCard from "../tools/LayoutCard.tsx";
import BarChartOutlinedIcon from "@mui/icons-material/BarChartOutlined";
import {Typography} from "@mui/material";
import EmptyState from "./EmptyState.tsx";
import { WidgetSwap, CascadeRowsLoader } from "../dashboard/widget-loaders";

function formatSampledAt(sampledAt: string): string {
  const parsed = new Date(sampledAt);
  return Number.isNaN(parsed.getTime()) ? '' : parsed.toLocaleString();
}

function TotalReadingsForEachSensorCard({ showTitle = true }: { showTitle?: boolean }) {
  const [sample, isLoading] = useTotalReadingsForEachSensor();

  const columns: GridColDef[] = [
    { field: 'sensor', headerName: 'Sensor', flex: 1 },
    { field: 'totalReadings', headerName: 'Total Readings', type: 'number', flex: 1 },
  ];

  const rows = Object.entries(sample.counts).map(([sensor, totalReadings], index) => ({
    id: index,
    sensor,
    totalReadings,
  }));

  const sampledAt = rows.length > 0 ? formatSampledAt(sample.sampled_at) : '';

  return (
    <LayoutCard variant="secondary" changes={{height: "100%", width: "100%"}}>
      <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', width: '100%', gap: 8 }}>
        {showTitle && <TypographyH2>Total Readings For Each Sensor</TypographyH2>}
        {sampledAt && (
          <Typography variant="caption" color="text.secondary" sx={{ marginLeft: 'auto' }}>
            Sampled {sampledAt}
          </Typography>
        )}
      </div>

      <div style={{ flex: 1, minHeight: 0, width: '100%' }}>
        <WidgetSwap loading={isLoading} loader={<CascadeRowsLoader />}>
          {rows.length === 0 ? (
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
        </WidgetSwap>
      </div>
    </LayoutCard>
  );
}

export default TotalReadingsForEachSensorCard;
