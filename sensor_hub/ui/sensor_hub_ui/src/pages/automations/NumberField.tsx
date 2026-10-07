import { useState } from 'react';
import { TextField } from '@mui/material';

interface NumberFieldProps {
  label: string;
  value: number | undefined;
  min?: number;
  step?: number;
  helperText?: string;
  readOnly: boolean;
  onChange: (value: number | undefined) => void;
}

export default function NumberField({ label, value, min, step, helperText, readOnly, onChange }: NumberFieldProps) {
  // Kept as typed, so that a partial number such as "15." is not rewritten from the number it gives.
  const [typed, setTyped] = useState(value === undefined ? '' : String(value));

  return (
    <TextField
      type="number"
      size="small"
      label={label}
      value={typed}
      helperText={helperText}
      disabled={readOnly}
      slotProps={{ htmlInput: { min, step } }}
      onChange={(event) => {
        setTyped(event.target.value);
        onChange(event.target.value === '' ? undefined : Number(event.target.value));
      }}
    />
  );
}
