import { DataGrid, type GridColDef } from "@mui/x-data-grid";
import { useCurrentReadings } from "../hooks/useCurrentReadings";
import { TypographyH2 } from "../tools/Typography";
import LayoutCard from "../tools/LayoutCard.tsx";
import { useIsMobile } from "../hooks/useMobile";
import { useSensorContext } from "../hooks/useSensorContext";
import EmptyState from "./EmptyState";
import ThermostatOutlinedIcon from "@mui/icons-material/ThermostatOutlined";
import { CascadeRowsLoader } from "../dashboard/widget-loaders";

interface CurrentTemperaturesProps {
  cardHeight?: string | number;
  showTitle?: boolean;
  onDataUpdate?: (date: Date) => void;
}

function CurrentTemperatures({ cardHeight, showTitle = true, onDataUpdate }: CurrentTemperaturesProps) {
  const isMobile = useIsMobile();
  const currentReadings = useCurrentReadings({ onDataUpdate });
  const { loaded } = useSensorContext();

  const sensorNames = Object.keys(currentReadings).sort((a, b) =>
    a.localeCompare(b)
  );

  const rows = sensorNames.flatMap((sensor) => {
    const byType = currentReadings[sensor];
    return Object.values(byType).map((reading) => ({
      id: `${sensor}:${reading.measurement_type}`,
      sensor_name: reading.sensor_name,
      value: reading.numeric_value,
      unit: reading.unit,
      measurement_type: reading.measurement_type,
      time: reading.time,
    }));
  });

  const columns: GridColDef[] = [
    { field: "sensor_name", headerName: "Sensor Name", flex: 1, minWidth: 150 },
    { field: "measurement_type", headerName: "Measurement", flex: 1, minWidth: 120 },
    {
      field: "value",
      headerName: "Value",
      flex: 1,
      type: "number",
      minWidth: 90,
      valueFormatter: (value: number | null, row: { unit?: string }) => {
        if (value == null) return '—';
        return `${value}${row.unit ? ` ${row.unit}` : ''}`;
      },
    },
    { field: "time", headerName: "Time", flex: 1, minWidth: 200 },
  ];

  const columnVisibilityModel = isMobile
    ? { measurement_type: false, time: false }
    : { measurement_type: true, time: true };

  return (
    <LayoutCard variant="secondary" changes={{ alignItems: "center", height: cardHeight, width: "100%" }}>
      {showTitle && <TypographyH2>Live Temperature</TypographyH2>}
      <div
        style={{
          flex: 1,
          minHeight: 0,
          display: "flex",
          flexDirection: "column",
          paddingBottom: 10,
          width: "100%"
        }}
      >
        {!loaded ? (
          <CascadeRowsLoader />
        ) : rows.length === 0 ? (
          <EmptyState
            icon={<ThermostatOutlinedIcon sx={{ fontSize: 48 }} />}
            title="No live temperature data"
            description="Add and enable sensors to see live readings here."
            actionLabel="Go to Sensors"
            actionHref="/sensors-overview"
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
            columnVisibilityModel={columnVisibilityModel}
            sx={{
              backgroundColor: 'background.paper',
              borderRadius: 2,
              mt: 2,
              '& .MuiDataGrid-cell': { fontSize: isMobile ? '0.9rem' : '1rem' },
              '& .MuiDataGrid-columnHeaders': { fontWeight: 'bold' },
            }}
          />
        )}
      </div>
    </LayoutCard>
  );
}

export default CurrentTemperatures;
