import CategoryOutlinedIcon from "@mui/icons-material/CategoryOutlined";
import { useSensorContext } from "../hooks/useSensorContext.ts";
import { useDrivers } from "../hooks/useDrivers.ts";
import SensorTypePieChart from "./SensorTypePieChart.tsx";
import Card from "../ui/Card";
import ChartArea from "../ui/ChartArea";
import EmptyState from "../ui/EmptyState";
import { CircularDrawLoader } from "../dashboard/widget-loaders";

function SensorTypeCard() {
  const { sensors, loaded } = useSensorContext();
  const { drivers } = useDrivers();

  return (
    <Card title="Sensor Types">
      {loaded && sensors.length === 0 ? (
        <EmptyState
          icon={<CategoryOutlinedIcon fontSize="large" />}
          title="No sensors to categorise"
          description="Sensor type breakdown will appear here once sensors are added."
        />
      ) : (
        <ChartArea size="md">
          {loaded ? <SensorTypePieChart sensors={sensors} drivers={drivers} /> : <CircularDrawLoader />}
        </ChartArea>
      )}
    </Card>
  );
}

export default SensorTypeCard;
