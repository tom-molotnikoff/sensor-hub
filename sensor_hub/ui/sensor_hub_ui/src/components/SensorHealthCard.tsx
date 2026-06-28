import {TypographyH2} from "../tools/Typography.tsx";
import SensorHealthPieChart from "./SensorHealthPieChart.tsx";
import LayoutCard from "../tools/LayoutCard.tsx";
import {useSensorContext} from "../hooks/useSensorContext.ts";
import { Box } from "@mui/material";
import EmptyState from "./EmptyState";
import MonitorHeartOutlinedIcon from "@mui/icons-material/MonitorHeartOutlined";
import { scrollToAndHighlight } from "../tools/scrollToAndHighlight";
import { CircularDrawLoader } from "../dashboard/widget-loaders";


function SensorHealthCard({ showTitle = true }: { showTitle?: boolean }) {
  const { sensors, loaded } = useSensorContext();

  return (
    <LayoutCard variant="secondary" changes={{alignItems: "center", height: "100%", width: "100%", overflow: "hidden"}}>
      {showTitle && <TypographyH2>Sensor Health</TypographyH2>}
      {!loaded ? (
        <Box sx={{ display: 'flex', justifyContent: 'center', alignItems: 'center', flex: 1, minHeight: 0, width: '100%' }}>
          <CircularDrawLoader />
        </Box>
      ) : sensors.length === 0 ? (
        <EmptyState
          icon={<MonitorHeartOutlinedIcon sx={{ fontSize: 48 }} />}
          title="No sensor health data"
          description="Add sensors to monitor their health status."
          actionLabel="Add a sensor"
          onAction={() => scrollToAndHighlight('add-sensor-form')}
        />
      ) : (
        <Box sx={{ flex: 1, minHeight: 0, width: '100%' }}>
          <SensorHealthPieChart sensors={sensors}/>
        </Box>
      )}
    </LayoutCard>
  )
}

export default SensorHealthCard;