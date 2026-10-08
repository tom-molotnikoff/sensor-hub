import { useState, useCallback } from 'react';
import type { Sensor } from '../gen/aliases';
import { apiClient } from '../gen/client';
import type { FormikHelpers } from 'formik';
import { logger } from '../tools/logger';
import type { SensorFormValues } from '../forms/SensorForm';
import { bestUnit, hoursToUnit, unitToHours } from '../tools/retention';

export interface UseSensorFormOpts {
  mode?: 'create' | 'edit';
  initialSensor?: Sensor | null;
  onSuccess?: (sensor: Sensor | null) => void;
}

function retentionInitialValues(sensor: Sensor | null): Pick<SensorFormValues, 'retentionEnabled' | 'retentionValue' | 'retentionUnit'> {
  if (sensor?.retention_hours != null) {
    const u = bestUnit(sensor.retention_hours);
    return { retentionEnabled: true, retentionValue: String(hoursToUnit(sensor.retention_hours, u)), retentionUnit: u };
  }
  return { retentionEnabled: false, retentionValue: '', retentionUnit: 'days' };
}

export function useSensorForm({ mode = 'edit', initialSensor = null, onSuccess }: UseSensorFormOpts) {
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [successMessage, setSuccessMessage] = useState<string | null>(null);
  const [errorMessage, setErrorMessage] = useState<string | null>(null);
  const [advancedErrorMessage, setAdvancedErrorMessage] = useState<string | null>(null);

  const initialValues: SensorFormValues = {
    name: initialSensor?.name ?? '',
    sensorDriver: initialSensor?.sensor_driver ?? '',
    config: initialSensor?.config ?? {},
    ...retentionInitialValues(initialSensor),
  };

  const handleErrors = (err: unknown) => {
    setErrorMessage(null);
    setAdvancedErrorMessage(null);
    if (err && typeof err === 'object' && 'message' in err) {
      setErrorMessage(String((err as { message: string }).message));
      if ('error' in (err as object)) setAdvancedErrorMessage(JSON.stringify((err as unknown as { error: unknown }).error));
      return;
    }
    if (err instanceof Error) {
      setErrorMessage(err.message);
      return;
    }
    try {
      setErrorMessage(JSON.stringify(err));
    } catch {
      setErrorMessage('Unknown error');
    }
  };

  const onSubmit = useCallback(
    async (values: SensorFormValues, actions: FormikHelpers<SensorFormValues>) => {
      setIsSubmitting(true);
      setSuccessMessage(null);
      setErrorMessage(null);
      setAdvancedErrorMessage(null);

      try {
        let result: { error?: unknown; response: Response };
        if (mode === 'create') {
          result = await apiClient.POST('/sensors', {
            body: { name: values.name, sensor_driver: values.sensorDriver, config: values.config } as never,
          });
        } else {
          if (!initialSensor || initialSensor.id == null) {
            throw new Error('Missing sensor id for update');
          }
          const retentionHours = values.retentionEnabled
            ? unitToHours(parseFloat(values.retentionValue), values.retentionUnit)
            : null;
          result = await apiClient.PUT('/sensors/{id}', {
            params: { path: { id: Number(initialSensor.id) } },
            body: { name: values.name, sensor_driver: values.sensorDriver, config: values.config, retention_hours: retentionHours } as never,
          });
        }
        if (!result.response.ok) {
          const { error, response } = result;
          handleErrors(error && typeof error === 'object' ? error : { message: `${response.status} ${response.statusText}` });
          return;
        }

        let newSensor: Sensor | null;
        try {
          const { data } = await apiClient.GET('/sensors/{name}', { params: { path: { name: values.name } } });
          newSensor = data ?? null;
        } catch (fetchErr) {
          logger.debug('Failed to fetch sensor after create/update:', fetchErr);
          newSensor = null;
        }

        setSuccessMessage(mode === 'create' ? 'Sensor created successfully!' : 'Sensor updated successfully!');
        setTimeout(() => setSuccessMessage(null), 3000);
        if (onSuccess) onSuccess(newSensor);

        if (actions && typeof actions.resetForm === 'function') {
          if (mode === 'create') {
            actions.resetForm({ values: { name: '', sensorDriver: '', config: {}, retentionEnabled: false, retentionValue: '', retentionUnit: 'days' } });
          } else {
            actions.resetForm({
              values: {
                name: newSensor?.name ?? values.name,
                sensorDriver: newSensor?.sensor_driver ?? values.sensorDriver,
                config: newSensor?.config ?? values.config,
                ...retentionInitialValues(newSensor),
              },
            });
          }
        }
      } catch (err) {
        handleErrors(err);
      } finally {
        setIsSubmitting(false);
        if (actions && typeof actions.setSubmitting === 'function') actions.setSubmitting(false);
      }
    },
    [mode, onSuccess, initialSensor]
  );

  return {
    initialValues,
    onSubmit,
    isSubmitting,
    successMessage,
    errorMessage,
    advancedErrorMessage,
    setSuccessMessage,
    setErrorMessage,
    setAdvancedErrorMessage,
  };
}