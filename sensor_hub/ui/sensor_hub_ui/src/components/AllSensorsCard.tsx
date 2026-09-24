import SensorsDataGrid from './SensorsDataGrid';
import { useSensorContext } from '../hooks/useSensorContext';
import { useAuth } from '../providers/AuthContext';
import { scrollToAndHighlight } from '../tools/scrollToAndHighlight';

export default function AllSensorsCard() {
  const { sensors } = useSensorContext();
  const { user } = useAuth();

  if (!user) return null;

  return (
    <SensorsDataGrid
      sensors={sensors}
      user={user}
      onAddSensor={() => scrollToAndHighlight('add-sensor-form')}
    />
  );
}
