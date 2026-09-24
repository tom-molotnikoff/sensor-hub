import type { Sensor } from "../gen/aliases";
import SensorForm from "../forms/SensorForm.tsx";
import { useAuth } from "../providers/AuthContext.tsx";
import Card from "../ui/Card";

interface EditSensorDetailsProps {
  sensor: Sensor
}

function EditSensorDetails({ sensor }: EditSensorDetailsProps) {
  const { user } = useAuth();

  return (
    <Card title="Edit Sensor Details">
      {user === undefined ? "Loading..." : <SensorForm sensor={sensor} mode="edit" user={user} />}
    </Card>
  );
}

export default EditSensorDetails;
