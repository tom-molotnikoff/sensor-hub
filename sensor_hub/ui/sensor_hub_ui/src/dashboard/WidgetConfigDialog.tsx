import { useState, useEffect, useMemo } from 'react';
import {
    Dialog, DialogTitle, DialogContent, DialogActions,
    Button, TextField, FormControlLabel, Switch,
    MenuItem, Select, InputLabel, FormControl, Checkbox, ListItemText,
} from '@mui/material';
import { DatePicker } from '@mui/x-date-pickers';
import { DateTime } from 'luxon';
import { getWidget } from './WidgetRegistry';
import { useDashboard } from './DashboardContext';
import { useSensorContext } from '../hooks/useSensorContext';
import { useSensorMeasurementTypes, useMeasurementTypesWithReadings } from '../hooks/useMeasurementTypes';
import { apiClient } from '../gen/client';
import type { MeasurementTypeInfo } from '../gen/aliases';
import { TIME_RANGE_PRESETS } from './timeRange';
import { getBinaryCapabilities, getControllableSensors, normalizeSensorToggleProperty } from './sensorToggleConfig';

const NO_MEASUREMENT_TYPES: MeasurementTypeInfo[] = [];

interface WidgetConfigDialogProps {
    open: boolean;
    widgetId: string | null;
    onClose: () => void;
}

export default function WidgetConfigDialog({ open, widgetId, onClose }: WidgetConfigDialogProps) {
    const { config, updateWidgetConfig } = useDashboard();
    const { sensors } = useSensorContext();
    const [localConfig, setLocalConfig] = useState<Record<string, unknown>>({});
    const [intersectedTypes, setIntersectedTypes] = useState<MeasurementTypeInfo[]>([]);

    const widget = widgetId ? config.widgets.find((w) => w.id === widgetId) : null;
    const definition = widget ? getWidget(widget.type) : null;

    const hasSensorSelect = definition?.configFields?.some(f => f.type === 'sensor-select') ?? false;
    const hasControllableSensorSelect = definition?.configFields?.some(f => f.type === 'controllable-sensor-select') ?? false;
    const hasMultiSensorSelect = definition?.configFields?.some(f => f.type === 'multi-sensor-select') ?? false;
    const hasBinaryCapabilitySelect = definition?.configFields?.some(f => f.type === 'binary-capability-select') ?? false;
    const hasMeasurementTypeSelect = definition?.configFields?.some(f => f.type === 'measurement-type-select') ?? false;

    const selectedSensorId = (localConfig.sensorId as number | undefined) ?? null;
    const selectedSensorIds = (Array.isArray(localConfig.sensorIds) ? localConfig.sensorIds : []) as number[];
    const selectedSensor = useMemo(
        () => (selectedSensorId ? sensors.find((sensor) => sensor.id === selectedSensorId) ?? null : null),
        [selectedSensorId, sensors],
    );
    const controllableSensors = useMemo(
        () => getControllableSensors(sensors),
        [sensors],
    );
    const binaryCapabilities = useMemo(
        () => getBinaryCapabilities(selectedSensor),
        [selectedSensor],
    );

    // Fetch measurement types based on context
    const sensorMT = useSensorMeasurementTypes(
        (hasSensorSelect || hasControllableSensorSelect) && hasMeasurementTypeSelect && selectedSensorId ? selectedSensorId : null
    );
    const globalMeasurementTypes = useMeasurementTypesWithReadings(open && hasMeasurementTypeSelect);

    // Multi-sensor intersection: fetch types for each selected sensor. The
    // fetched list only shows while the multi-sensor selection is active.
    const selectedSensorIdsKey = selectedSensorIds.join(',');
    const showIntersection = hasMultiSensorSelect && hasMeasurementTypeSelect && selectedSensorIds.length > 0;
    useEffect(() => {
        const ids = selectedSensorIdsKey ? selectedSensorIdsKey.split(',').map(Number) : [];
        if (!hasMultiSensorSelect || !hasMeasurementTypeSelect || ids.length === 0) return;
        Promise.all(ids.map(id => apiClient.GET('/sensors/by-id/{id}/measurement-types', { params: { path: { id } } }).then(({ data }) => (data as MeasurementTypeInfo[] | null) ?? [])))
            .then(results => {
                if (results.length === 0) { setIntersectedTypes([]); return; }
                const sets = results.map(r => new Set(r.map(mt => mt.name)));
                const common = results[0].filter(mt => sets.every(s => s.has(mt.name)));
                setIntersectedTypes(common);
            })
            .catch(() => setIntersectedTypes([]));
    }, [hasMultiSensorSelect, hasMeasurementTypeSelect, selectedSensorIdsKey]);

    // Determine which measurement type list to display
    const filteredMeasurementTypes =
        (hasSensorSelect || hasControllableSensorSelect) && selectedSensorId ? sensorMT.measurementTypes
        : showIntersection ? intersectedTypes
        : hasMeasurementTypeSelect ? globalMeasurementTypes
        : NO_MEASUREMENT_TYPES;

    // Clear a measurement type that is no longer offered (adjust-during-render).
    const currentMT = localConfig.measurementType as string | undefined;
    if (currentMT && filteredMeasurementTypes.length > 0 && !filteredMeasurementTypes.some(mt => mt.name === currentMT)) {
        setLocalConfig(prev => ({ ...prev, measurementType: '' }));
    }

    // Keep the binary property aligned with the selected sensor's capabilities
    // (adjust-during-render).
    if (hasBinaryCapabilitySelect) {
        if (binaryCapabilities.length === 0) {
            if (localConfig.property) {
                setLocalConfig(prev => ({ ...prev, property: '' }));
            }
        } else {
            const normalizedProperty = normalizeSensorToggleProperty(localConfig.property, binaryCapabilities);
            if (normalizedProperty !== localConfig.property) {
                setLocalConfig(prev => ({ ...prev, property: normalizedProperty }));
            }
        }
    }

    // Re-seed the editable copy when a different widget is opened (adjust-during-render).
    const [prevWidget, setPrevWidget] = useState(widget);
    if (prevWidget !== widget) {
        setPrevWidget(widget);
        if (widget) setLocalConfig({ ...widget.config });
    }

    if (!widget || !definition?.configFields?.length) return null;

    const handleSave = () => {
        if (!widgetId) return;

        const nextConfig = definition.type === 'sensor-toggle'
            ? { ...localConfig, property: normalizeSensorToggleProperty(localConfig.property, binaryCapabilities) }
            : localConfig;

        updateWidgetConfig(widgetId, nextConfig);
        onClose();
    };

    return (
        <Dialog open={open} onClose={onClose} maxWidth="xs" fullWidth>
            <DialogTitle>Configure {definition.label}</DialogTitle>
            <DialogContent sx={{ display: 'flex', flexDirection: 'column', gap: 2 }}>
                {definition.configFields.map((field) => {
                    const value = localConfig[field.key] ?? field.defaultValue ?? '';

                    switch (field.type) {
                        case 'text':
                            return (
                                <TextField
                                    key={field.key} label={field.label} fullWidth
                                    sx={{ mt: 1 }}
                                    value={value as string}
                                    onChange={(e) => setLocalConfig({ ...localConfig, [field.key]: e.target.value })}
                                />
                            );
                        case 'textarea':
                            return (
                                <TextField
                                    key={field.key} label={field.label} fullWidth multiline minRows={3} maxRows={10}
                                    sx={{ mt: 1 }}
                                    value={value as string}
                                    onChange={(e) => setLocalConfig({ ...localConfig, [field.key]: e.target.value })}
                                />
                            );
                        case 'number':
                            return (
                                <TextField
                                    key={field.key} label={field.label} fullWidth type="number"
                                    sx={{ mt: 1 }}
                                    value={value as number}
                                    onChange={(e) => setLocalConfig({ ...localConfig, [field.key]: Number(e.target.value) })}
                                />
                            );
                        case 'boolean':
                            return (
                                <FormControlLabel
                                    key={field.key}
                                    sx={{ mt: 1 }}
                                    control={
                                        <Switch
                                            checked={Boolean(value)}
                                            onChange={(e) => setLocalConfig({ ...localConfig, [field.key]: e.target.checked })}
                                        />
                                    }
                                    label={field.label}
                                />
                            );
                        case 'select':
                            return (
                                <FormControl sx={{ mt: 1 }} key={field.key} fullWidth>
                                    <InputLabel>{field.label}</InputLabel>
                                    <Select
                                        value={value as string} label={field.label}
                                        onChange={(e) => setLocalConfig({ ...localConfig, [field.key]: e.target.value })}
                                    >
                                        {field.options?.map((opt) => (
                                            <MenuItem key={opt.value} value={opt.value}>{opt.label}</MenuItem>
                                        ))}
                                    </Select>
                                </FormControl>
                            );
                        case 'sensor-select':
                        case 'controllable-sensor-select': {
                            const selectableSensors = field.type === 'controllable-sensor-select' ? controllableSensors : sensors;
                            return (
                                <FormControl sx={{ mt: 1 }} key={field.key} fullWidth>
                                    <InputLabel>{field.label}</InputLabel>
                                    <Select
                                        value={(value as number) || ''} label={field.label}
                                        onChange={(e) => setLocalConfig({ ...localConfig, [field.key]: Number(e.target.value) })}
                                    >
                                        {selectableSensors.map((s) => (
                                            <MenuItem key={s.id} value={s.id}>{s.name}</MenuItem>
                                        ))}
                                    </Select>
                                </FormControl>
                            );
                        }
                        case 'binary-capability-select':
                            return (
                                <FormControl sx={{ mt: 1 }} key={field.key} fullWidth disabled={selectedSensor == null || binaryCapabilities.length === 0}>
                                    <InputLabel>{field.label}</InputLabel>
                                    <Select
                                        value={(value as string) || ''} label={field.label}
                                        onChange={(e) => setLocalConfig({ ...localConfig, [field.key]: e.target.value })}
                                    >
                                        {binaryCapabilities.map((capability) => (
                                            <MenuItem key={capability.property} value={capability.property}>{capability.property}</MenuItem>
                                        ))}
                                    </Select>
                                </FormControl>
                            );
                        case 'multi-sensor-select': {
                            const selected = (Array.isArray(value) ? value : []) as number[];
                            return (
                                <FormControl sx={{ mt: 1 }} key={field.key} fullWidth>
                                    <InputLabel>{field.label}</InputLabel>
                                    <Select<number[]>
                                        multiple
                                        value={selected}
                                        label={field.label}
                                        onChange={(e) => {
                                            const val = e.target.value;
                                            setLocalConfig({ ...localConfig, [field.key]: typeof val === 'string' ? val.split(',').map(Number) : val });
                                        }}
                                        renderValue={(sel) => sensors.filter((s) => sel.includes(s.id)).map((s) => s.name).join(', ')}
                                    >
                                        {sensors.map((s) => (
                                            <MenuItem key={s.id} value={s.id}>
                                                <Checkbox checked={selected.includes(s.id)} />
                                                <ListItemText primary={s.name} />
                                            </MenuItem>
                                        ))}
                                    </Select>
                                </FormControl>
                            );
                        }
                        case 'date': {
                            const dt = typeof value === 'string' && value
                                ? DateTime.fromISO(value as string)
                                : null;
                            return (
                                <DatePicker
                                    key={field.key}
                                    label={field.label}
                                    value={dt}
                                    onChange={(newVal: DateTime | null) => {
                                        setLocalConfig({
                                            ...localConfig,
                                            [field.key]: newVal?.toISODate() ?? '',
                                        });
                                    }}
                                    slotProps={{ textField: { fullWidth: true, sx: { mt: 1 } } }}
                                />
                            );
                        }
                        case 'time-range': {
                            const rangeValue = (localConfig.timeRange as string) || '24h';
                            const isCustom = rangeValue === 'custom';
                            const customStart = typeof localConfig.customStart === 'string' && localConfig.customStart
                                ? DateTime.fromISO(localConfig.customStart) : null;
                            const customEnd = typeof localConfig.customEnd === 'string' && localConfig.customEnd
                                ? DateTime.fromISO(localConfig.customEnd) : null;
                            return (
                                <div key={field.key}>
                                    <FormControl sx={{ mt: 1 }} fullWidth>
                                        <InputLabel>{field.label}</InputLabel>
                                        <Select
                                            value={rangeValue}
                                            label={field.label}
                                            onChange={(e) => setLocalConfig({ ...localConfig, timeRange: e.target.value })}
                                        >
                                            {TIME_RANGE_PRESETS.map((p) => (
                                                <MenuItem key={p.value} value={p.value}>{p.label}</MenuItem>
                                            ))}
                                        </Select>
                                    </FormControl>
                                    {isCustom && (
                                        <>
                                            <DatePicker
                                                label="Start Date"
                                                value={customStart}
                                                onChange={(v: DateTime | null) =>
                                                    setLocalConfig({ ...localConfig, customStart: v?.toISODate() ?? '' })
                                                }
                                                slotProps={{ textField: { fullWidth: true, sx: { mt: 1 } } }}
                                            />
                                            <DatePicker
                                                label="End Date"
                                                value={customEnd}
                                                onChange={(v: DateTime | null) =>
                                                    setLocalConfig({ ...localConfig, customEnd: v?.toISODate() ?? '' })
                                                }
                                                slotProps={{ textField: { fullWidth: true, sx: { mt: 1 } } }}
                                            />
                                        </>
                                    )}
                                </div>
                            );
                        }
                        case 'measurement-type-select':
                            return (
                                <FormControl sx={{ mt: 1 }} key={field.key} fullWidth>
                                    <InputLabel>{field.label}</InputLabel>
                                    <Select
                                        value={(value as string) || ''} label={field.label}
                                        onChange={(e) => setLocalConfig({ ...localConfig, [field.key]: e.target.value, aggregationFunction: '' })}
                                    >
                                        {filteredMeasurementTypes.map((mt) => (
                                            <MenuItem key={mt.name} value={mt.name}>{mt.display_name} ({mt.unit})</MenuItem>
                                        ))}
                                    </Select>
                                </FormControl>
                            );
                        case 'aggregation-function-select': {
                            const selectedMT = localConfig.measurementType as string | undefined;
                            const mtInfo = selectedMT ? filteredMeasurementTypes.find(mt => mt.name === selectedMT) : null;
                            const supported = mtInfo?.supported_aggregation_functions ?? [];
                            const labels: Record<string, string> = { avg: 'Average', min: 'Minimum', max: 'Maximum', sum: 'Sum', count: 'Count', last: 'Last' };
                            return (
                                <FormControl sx={{ mt: 1 }} key={field.key} fullWidth disabled={supported.length === 0}>
                                    <InputLabel>{field.label}</InputLabel>
                                    <Select
                                        value={(value as string) || ''} label={field.label}
                                        onChange={(e) => setLocalConfig({ ...localConfig, [field.key]: e.target.value })}
                                    >
                                        <MenuItem value="">Auto (default for type)</MenuItem>
                                        {supported.map((fn) => (
                                            <MenuItem key={fn} value={fn}>{labels[fn] ?? fn}</MenuItem>
                                        ))}
                                    </Select>
                                </FormControl>
                            );
                        }
                        default:
                            return null;
                    }
                })}
            </DialogContent>
            <DialogActions>
                <Button onClick={onClose}>Cancel</Button>
                <Button variant="contained" onClick={handleSave}>Save</Button>
            </DialogActions>
        </Dialog>
    );
}
