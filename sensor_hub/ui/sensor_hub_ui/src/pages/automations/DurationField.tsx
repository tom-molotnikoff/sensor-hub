import { useState } from 'react';
import { MenuItem, TextField } from '@mui/material';

export interface DurationUnit {
  name: string;
  seconds: number;
}

interface DurationFieldProps {
  label: string;
  seconds: number | undefined;
  units: readonly DurationUnit[];
  min?: number;
  readOnly: boolean;
  onChange: (seconds: number | undefined) => void;
}

const largestWholeUnit = (seconds: number | undefined, units: readonly DurationUnit[]) =>
  [...units].reverse().find((unit) => seconds !== undefined && seconds !== 0 && seconds % unit.seconds === 0)?.seconds ?? units[0].seconds;

export default function DurationField({ label, seconds, units, min = 1, readOnly, onChange }: DurationFieldProps) {
  const [unit, setUnit] = useState<number>(() => largestWholeUnit(seconds, units));
  // Kept as typed, so that a partial number such as "1." is not rewritten from the seconds it gives.
  const [amount, setAmount] = useState(() => (seconds === undefined ? '' : String(seconds / unit)));
  const setSeconds = (typed: string, inUnit: number) => onChange(typed === '' ? undefined : Math.round(Number(typed) * inUnit));

  return (
    <>
      <TextField
        type="number"
        size="small"
        label={label}
        value={amount}
        disabled={readOnly}
        slotProps={{ htmlInput: { min } }}
        onChange={(event) => {
          setAmount(event.target.value);
          setSeconds(event.target.value, unit);
        }}
      />
      <TextField
        select
        size="small"
        label="Unit"
        value={unit}
        disabled={readOnly}
        onChange={(event) => {
          const chosen = Number(event.target.value);
          setUnit(chosen);
          setSeconds(amount, chosen);
        }}
      >
        {units.map((each) => (
          <MenuItem key={each.name} value={each.seconds}>
            {each.name}
          </MenuItem>
        ))}
      </TextField>
    </>
  );
}
