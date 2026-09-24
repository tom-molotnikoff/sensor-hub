import MonitorHeartOutlinedIcon from "@mui/icons-material/MonitorHeartOutlined";
import SensorHealthPieChart from "./SensorHealthPieChart.tsx";
import { useSensorContext } from "../hooks/useSensorContext.ts";
import Card from "../ui/Card";
import ChartArea from "../ui/ChartArea";
import EmptyState from "../ui/EmptyState";
import { scrollToAndHighlight } from "../tools/scrollToAndHighlight";
import { CircularDrawLoader } from "../dashboard/widget-loaders";

function SensorHealthCard() {
  const { sensors, loaded } = useSensorContext();

  return (
    <Card title="Sensor Health">
      {loaded && sensors.length === 0 ? (
        <EmptyState
          icon={<MonitorHeartOutlinedIcon fontSize="large" />}
          title="No sensor health data"
          description="Add sensors to monitor their health status."
          actionLabel="Add a sensor"
          onAction={() => scrollToAndHighlight('add-sensor-form')}
        />
      ) : (
        <ChartArea size="md">
          {loaded ? <SensorHealthPieChart sensors={sensors} /> : <CircularDrawLoader />}
        </ChartArea>
      )}
    </Card>
  );
}

export default SensorHealthCard;
