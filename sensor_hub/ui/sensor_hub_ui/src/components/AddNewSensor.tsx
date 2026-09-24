import SensorForm from "../forms/SensorForm.tsx";
import {useAuth} from "../providers/AuthContext.tsx";
import Card from "../ui/Card";

function AddNewSensor() {
  const { user } = useAuth();

  return (
    <Card id="add-sensor-form" title="Add Sensor">
      {user === undefined ? 'Loading...' : <SensorForm mode="create" user={ user }/>}
    </Card>
  )
}

export default AddNewSensor;
