import { TypographyH2 } from "../tools/Typography.tsx";
import LayoutCard from "../tools/LayoutCard.tsx";
import { useSensorContext } from "../hooks/useSensorContext.ts";
import { useDrivers } from "../hooks/useDrivers.ts";
import SensorTypePieChart from "./SensorTypePieChart.tsx";
import { Box } from "@mui/material";
import EmptyState from "./EmptyState";
import CategoryOutlinedIcon from "@mui/icons-material/CategoryOutlined";
import { CircularDrawLoader } from "../dashboard/widget-loaders";

function SensorTypeCard({ showTitle = true }: { showTitle?: boolean }) {
  const { sensors, loaded } = useSensorContext();
  const { drivers } = useDrivers();

  return (
    <LayoutCard
      variant="secondary"
      changes={{ alignItems: "center", height: "100%", width: "100%", overflow: "hidden" }}
    >
      {showTitle && <TypographyH2>Sensor Types</TypographyH2>}
      {!loaded ? (
        <Box sx={{ display: 'flex', justifyContent: 'center', alignItems: 'center', flex: 1, minHeight: 0, width: '100%' }}>
          <CircularDrawLoader />
        </Box>
      ) : sensors.length === 0 ? (
        <EmptyState
          icon={<CategoryOutlinedIcon sx={{ fontSize: 48 }} />}
          title="No sensors to categorise"
          description="Sensor type breakdown will appear here once sensors are added."
        />
      ) : (
        <Box sx={{ flex: 1, minHeight: 0, width: '100%' }}>
          <SensorTypePieChart sensors={sensors} drivers={drivers} />
        </Box>
      )}
    </LayoutCard>
  );
}

export default SensorTypeCard;
