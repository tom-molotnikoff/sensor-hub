import { useState } from 'react';
import { Dialog, DialogTitle, DialogContent, DialogActions, Button, IconButton } from '@mui/material';
import { DatePicker } from '@mui/x-date-pickers';
import { DateTime } from 'luxon';
import SettingsIcon from '@mui/icons-material/Settings';
import ReadingsChart from './ReadingsChart';
import type { Sensor } from '../gen/aliases';
import Card from '../ui/Card';
import Stack from '../ui/Stack';

interface SensorTemperatureDataCardProps {
  sensor: Sensor;
}

export default function SensorTemperatureDataCard({ sensor }: SensorTemperatureDataCardProps) {
  const [startDate, setStartDate] = useState<DateTime | null>(DateTime.now().startOf('day'));
  const [endDate, setEndDate] = useState<DateTime | null>(DateTime.now().plus({ days: 1 }).startOf('day'));

  const [settingsOpen, setSettingsOpen] = useState(false);
  const [draftStart, setDraftStart] = useState<DateTime | null>(startDate);
  const [draftEnd, setDraftEnd] = useState<DateTime | null>(endDate);

  if (sensor.sensor_driver !== 'sensor-hub-http-temperature') return null;

  const handleOpen = () => {
    setDraftStart(startDate);
    setDraftEnd(endDate);
    setSettingsOpen(true);
  };

  const handleSave = () => {
    setStartDate(draftStart);
    setEndDate(draftEnd);
    setSettingsOpen(false);
  };

  return (
    <Card
      title="Indoor Temperature Data"
      actions={
        <IconButton onClick={handleOpen} size="small" title="Settings">
          <SettingsIcon />
        </IconButton>
      }
    >
      <ReadingsChart sensors={[sensor]} startDate={startDate} endDate={endDate} />
      <Dialog open={settingsOpen} onClose={() => setSettingsOpen(false)} maxWidth="xs" fullWidth>
        <DialogTitle>Temperature Chart Settings</DialogTitle>
        <DialogContent>
          <Stack>
            <DatePicker
              label="Start Date"
              value={draftStart}
              onChange={setDraftStart}
              slotProps={{ textField: { fullWidth: true } }}
            />
            <DatePicker
              label="End Date"
              value={draftEnd}
              onChange={setDraftEnd}
              slotProps={{ textField: { fullWidth: true } }}
            />
          </Stack>
        </DialogContent>
        <DialogActions>
          <Button onClick={() => setSettingsOpen(false)}>Cancel</Button>
          <Button variant="contained" onClick={handleSave}>Apply</Button>
        </DialogActions>
      </Dialog>
    </Card>
  );
}
