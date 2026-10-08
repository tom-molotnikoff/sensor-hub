import { act, renderHook } from '@testing-library/react';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import type { FormikHelpers } from 'formik';
import type { Sensor } from '../gen/aliases';
import type { SensorFormValues } from '../forms/SensorForm';
import { useSensorForm } from './useSensorForm';

const { getMock, postMock, putMock } = vi.hoisted(() => ({
  getMock: vi.fn(),
  postMock: vi.fn(),
  putMock: vi.fn(),
}));

vi.mock('../gen/client', () => ({
  apiClient: { GET: getMock, POST: postMock, PUT: putMock },
}));

const values: SensorFormValues = {
  name: 'kitchen',
  sensorDriver: 'sensor-hub-http-temperature',
  config: { url: 'http://kitchen.local' },
  retentionEnabled: false,
  retentionValue: '',
  retentionUnit: 'days',
};

const lounge: Sensor = { id: 2, name: 'lounge', sensor_driver: 'sensor-hub-http-temperature', config: {}, enabled: true } as Sensor;

function refused(status: number, statusText: string, error: unknown) {
  return { error, response: new Response(null, { status, statusText }) };
}

function formikActions() {
  return { resetForm: vi.fn(), setSubmitting: vi.fn() } as unknown as FormikHelpers<SensorFormValues>;
}

describe('useSensorForm', () => {
  beforeEach(() => {
    getMock.mockReset();
    postMock.mockReset();
    putMock.mockReset();
  });

  it('shows the API message on a create 409 and keeps the entered values', async () => {
    postMock.mockResolvedValue(refused(409, 'Conflict', { message: 'sensor with name kitchen already exists' }));
    const onSuccess = vi.fn();
    const actions = formikActions();
    const { result } = renderHook(() => useSensorForm({ mode: 'create', onSuccess }));

    await act(() => result.current.onSubmit(values, actions));

    expect(result.current.errorMessage).toBe('sensor with name kitchen already exists');
    expect(result.current.advancedErrorMessage).toBeNull();
    expect(result.current.successMessage).toBeNull();
    expect(onSuccess).not.toHaveBeenCalled();
    expect(actions.resetForm).not.toHaveBeenCalled();
    expect(getMock).not.toHaveBeenCalled();
  });

  it('shows the API message on an edit 409', async () => {
    putMock.mockResolvedValue(refused(409, 'Conflict', { message: 'sensor with name kitchen already exists' }));
    const onSuccess = vi.fn();
    const actions = formikActions();
    const { result } = renderHook(() => useSensorForm({ mode: 'edit', initialSensor: lounge, onSuccess }));

    await act(() => result.current.onSubmit(values, actions));

    expect(result.current.errorMessage).toBe('sensor with name kitchen already exists');
    expect(result.current.successMessage).toBeNull();
    expect(onSuccess).not.toHaveBeenCalled();
    expect(actions.resetForm).not.toHaveBeenCalled();
    expect(getMock).not.toHaveBeenCalled();
  });

  it('shows the error field as the detail line on a 500', async () => {
    postMock.mockResolvedValue(refused(500, 'Internal Server Error', { message: 'Error adding sensor', error: 'database is locked' }));
    const { result } = renderHook(() => useSensorForm({ mode: 'create' }));

    await act(() => result.current.onSubmit(values, formikActions()));

    expect(result.current.errorMessage).toBe('Error adding sensor');
    expect(result.current.advancedErrorMessage).toBe('"database is locked"');
    expect(result.current.successMessage).toBeNull();
  });

  it('shows the status when a refused response has no parseable body', async () => {
    postMock.mockResolvedValue(refused(502, 'Bad Gateway', undefined));
    const { result } = renderHook(() => useSensorForm({ mode: 'create' }));

    await act(() => result.current.onSubmit(values, formikActions()));

    expect(result.current.errorMessage).toBe('502 Bad Gateway');
    expect(result.current.successMessage).toBeNull();
  });

  it('reports success, calls onSuccess and resets the form on a 2xx', async () => {
    postMock.mockResolvedValue({ data: { message: 'Sensor added successfully' }, response: new Response(null, { status: 201 }) });
    const created = { ...lounge, id: 3, name: 'kitchen' };
    getMock.mockResolvedValue({ data: created });
    const onSuccess = vi.fn();
    const actions = formikActions();
    const { result } = renderHook(() => useSensorForm({ mode: 'create', onSuccess }));

    await act(() => result.current.onSubmit(values, actions));

    expect(result.current.successMessage).toBe('Sensor created successfully!');
    expect(result.current.errorMessage).toBeNull();
    expect(onSuccess).toHaveBeenCalledWith(created);
    expect(actions.resetForm).toHaveBeenCalled();
  });
});
