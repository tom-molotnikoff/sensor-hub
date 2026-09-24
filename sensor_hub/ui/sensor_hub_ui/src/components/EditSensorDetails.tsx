import type {Sensor} from "../gen/aliases";
import SensorForm from "../forms/SensorForm.tsx";
import LayoutCard from "../tools/LayoutCard.tsx";
import {useAuth} from "../providers/AuthContext.tsx";
import {TypographyH2} from "../tools/Typography.tsx";


interface EditSensorDetailsProps {
  sensor: Sensor
}


function EditSensorDetails({ sensor } : EditSensorDetailsProps) {
  const { user } = useAuth();
  if (user === undefined) {
    return (
      <LayoutCard variant={"secondary"} changes={{height: "100%", width: "100%", minHeight: 400}}>
        Loading...
      </LayoutCard>
    )
  }

  return (
    <LayoutCard variant={"secondary"} changes={{height: "100%", width: "100%", minHeight: 400}}>
      <TypographyH2>Edit Sensor Details</TypographyH2>
      <SensorForm sensor={sensor} mode="edit" user={user} />
    </LayoutCard>
  )
}

export default EditSensorDetails;