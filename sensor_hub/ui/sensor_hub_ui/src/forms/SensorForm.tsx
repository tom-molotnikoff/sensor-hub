import type {Sensor, DriverInfo} from "../gen/aliases";
import {Formik, Form, type FormikProps} from 'formik';
import { Button, Box, Stack, TextField, Typography, Alert, MenuItem, Divider, FormControlLabel, Switch } from '@mui/material';
import {useSensorForm} from "../hooks/useSensorForm.ts";
import {useDrivers} from "../hooks/useDrivers.ts";
import * as Yup from 'yup';
import type {AuthUser} from "../providers/AuthContext.tsx";
import {hasPerm} from "../tools/Utils.ts";
import {TypographyH2} from "../tools/Typography.tsx";
import {useProperties} from "../hooks/useProperties.ts";
import {formatRetention, unitToHours, hoursToUnit, type RetentionUnit} from "../tools/retention.ts";

interface SensorFormProps {
  sensor?: Sensor;
  mode?: 'create' | 'edit';
  onSuccess?: (sensor: Sensor | null) => void;
  user: AuthUser;
}

export type SensorFormValues = {
  name: string;
  sensorDriver: string;
  config: Record<string, string>;
  retentionEnabled: boolean;
  retentionValue: string;
  retentionUnit: RetentionUnit;
};

function buildValidationSchema(selectedDriver: DriverInfo | undefined) {
  const shape: Record<string, Yup.Schema> = {
    name: Yup.string().required('Name is required'),
    sensorDriver: Yup.string().required('Sensor driver is required'),
  };
  if (selectedDriver) {
    const configShape: Record<string, Yup.StringSchema> = {};
    for (const field of selectedDriver.config_fields) {
      let s = Yup.string();
      if (field.required) {
        s = s.required(`${field.label} is required`);
      }
      configShape[field.key] = s;
    }
    shape.config = Yup.object().shape(configShape);
  }
  return Yup.object().shape(shape);
}

function SensorForm ({ sensor, mode = 'edit', onSuccess, user } : SensorFormProps) {
  const { drivers } = useDrivers('pull');
  const properties = useProperties();
  const globalRetentionDays = parseInt(properties['sensor.data.retention.days'] || '90', 10);
  const globalRetentionHours = globalRetentionDays * 24;
  const {
    initialValues,
    onSubmit,
    isSubmitting,
    successMessage,
    errorMessage,
    advancedErrorMessage,
    setSuccessMessage,
    setErrorMessage,
    setAdvancedErrorMessage,
  } = useSensorForm({ mode, initialSensor: sensor ?? null, onSuccess });

  const fieldsDisabled = !(hasPerm(user, "manage_sensors"));

  return (
    <>
      <TypographyH2>
        {mode === 'create' ? 'Add Sensor' : 'Edit Sensor Details'}
      </TypographyH2>
      <Formik<SensorFormValues>
        initialValues={initialValues}
        enableReinitialize onSubmit={onSubmit}
        validationSchema={buildValidationSchema(drivers.find(d => d.type === initialValues.sensorDriver))}
      >
        {(formik: FormikProps<SensorFormValues>) => {
          const { isSubmitting: formikSubmitting, dirty, errors, touched, values, setFieldValue } = formik;

          const selectedDriver = drivers.find(d => d.type === values.sensorDriver);

          const pendingEffectiveHours = values.retentionEnabled && values.retentionValue
            ? unitToHours(parseFloat(values.retentionValue), values.retentionUnit)
            : globalRetentionHours;

          return (
            <Form>
              <Stack spacing={2}>
                <TextField
                  name="name"
                  label="Name"
                  variant="outlined"
                  fullWidth
                  size="small"
                  value={values.name}
                  onChange={formik.handleChange}
                  disabled={fieldsDisabled}
                  error={!!(errors.name && touched.name)}
                  helperText={touched.name && errors.name}
                />

                <TextField
                  name="sensorDriver"
                  label="Sensor Driver"
                  variant="outlined"
                  fullWidth
                  select
                  size="small"
                  disabled={fieldsDisabled}
                  value={values.sensorDriver}
                  onChange={(e) => {
                    const newDriver = e.target.value;
                    void setFieldValue('sensorDriver', newDriver);
                    const driver = drivers.find(d => d.type === newDriver);
                    if (driver) {
                      const newConfig: Record<string, string> = {};
                      for (const f of driver.config_fields) {
                        newConfig[f.key] = f.default ?? '';
                      }
                      void setFieldValue('config', newConfig);
                    } else {
                      void setFieldValue('config', {});
                    }
                  }}
                  error={!!(errors.sensorDriver && touched.sensorDriver)}
                  helperText={touched.sensorDriver && errors.sensorDriver}
                >
                  {drivers.map((driver) => (
                    <MenuItem key={driver.type} value={driver.type}>
                      {driver.display_name}
                    </MenuItem>
                  ))}
                </TextField>

                {selectedDriver?.config_fields.map((field) => {
                  const configErrors = errors.config as Record<string, string> | undefined;
                  const configTouched = touched.config as Record<string, boolean> | undefined;
                  return (
                    <TextField
                      key={field.key}
                      name={`config.${field.key}`}
                      label={field.label}
                      helperText={
                        (configTouched?.[field.key] && configErrors?.[field.key])
                          ? configErrors[field.key]
                          : field.description
                      }
                      error={!!(configTouched?.[field.key] && configErrors?.[field.key])}
                      type={field.sensitive ? 'password' : 'text'}
                      variant="outlined"
                      fullWidth
                      size="small"
                      required={field.required}
                      value={values.config?.[field.key] ?? field.default ?? ''}
                      onChange={formik.handleChange}
                      disabled={fieldsDisabled}
                    />
                  );
                })}

                {mode === 'edit' && (
                  <>
                    <Divider />
                    <Typography variant="subtitle2" sx={{
                      color: "text.secondary"
                    }}>
                      Effective retention: <strong>{formatRetention(pendingEffectiveHours)}</strong>
                      {' '}(global default: {formatRetention(globalRetentionHours)})
                    </Typography>
                    <FormControlLabel
                      control={
                        <Switch
                          checked={values.retentionEnabled}
                          onChange={(e) => {
                            void setFieldValue('retentionEnabled', e.target.checked);
                            if (!e.target.checked) void setFieldValue('retentionValue', '');
                          }}
                          disabled={fieldsDisabled}
                        />
                      }
                      label="Override global data retention"
                    />
                    {values.retentionEnabled && (
                      <Box sx={{ display: 'flex', gap: 2, alignItems: 'flex-start' }}>
                        <TextField
                          label="Retention"
                          type="number"
                          value={values.retentionValue}
                          onChange={(e) => void setFieldValue('retentionValue', e.target.value)}
                          disabled={fieldsDisabled}
                          slotProps={{ htmlInput: { min: 1, step: 1 } }}
                          size="small"
                          sx={{ flex: 1 }}
                        />
                        <TextField
                          select
                          label="Unit"
                          value={values.retentionUnit}
                          onChange={(e) => {
                            const newUnit = e.target.value as RetentionUnit;
                            if (values.retentionValue) {
                              const hours = unitToHours(parseFloat(values.retentionValue), values.retentionUnit);
                              void setFieldValue('retentionValue', String(hoursToUnit(hours, newUnit)));
                            }
                            void setFieldValue('retentionUnit', newUnit);
                          }}
                          disabled={fieldsDisabled}
                          size="small"
                          sx={{ minWidth: 120 }}
                        >
                          <MenuItem value="hours">Hours</MenuItem>
                          <MenuItem value="days">Days</MenuItem>
                          <MenuItem value="weeks">Weeks</MenuItem>
                        </TextField>
                      </Box>
                    )}
                  </>
                )}

                <Box sx={{
                  display: "flex"
                }}>
                  <Button
                    type="reset"
                    disabled={isSubmitting || formikSubmitting || fieldsDisabled}
                    variant="outlined"
                    color="primary"
                    sx={{ mr: 2 }}
                  >
                    Reset
                  </Button>
                  <Button
                    type="submit"
                    disabled={isSubmitting || formikSubmitting || !dirty || fieldsDisabled}
                    variant="contained"
                    color="primary"

                  >
                    {mode === 'create' ? 'Create' : 'Submit'}
                  </Button>
                </Box>
              </Stack>
            </Form>
          );
        }}
      </Formik>
      {successMessage && (
        <Box sx={{
          mt: 2
        }}>
          <Alert severity="success" onClose={() => setSuccessMessage(null)}>
            {successMessage}
          </Alert>
        </Box>
      )}
      {errorMessage && (
        <Box sx={{
          mt: 2
        }}>
          <Alert severity="error" onClose={() => { setErrorMessage(null); setAdvancedErrorMessage(null); }}>
            {errorMessage}
            {advancedErrorMessage && (
              <Box
                sx={{
                  mt: 1,
                  whiteSpace: 'pre-wrap',
                  fontFamily: 'monospace',
                  fontSize: '0.75rem'
                }}>
                {advancedErrorMessage}
              </Box>
            )}
          </Alert>
        </Box>
      )}
    </>
  );
}

export default SensorForm;

