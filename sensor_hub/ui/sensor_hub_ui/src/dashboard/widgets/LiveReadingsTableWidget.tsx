import ThermostatOutlinedIcon from '@mui/icons-material/ThermostatOutlined';
import type { WidgetProps } from '../types';
import { useCurrentReadings, useCurrentReadingsReady } from '../../hooks/useCurrentReadings';
import { useSensorContext } from '../../hooks/useSensorContext';
import { useReportWidgetUpdate } from '../WidgetUpdateContext';
import { useWidgetStateReport } from '../WidgetContext';
import DataTable, { type DataTableColumn } from '../../ui/DataTable';
import EmptyState from '../../ui/EmptyState';
import { WidgetSwap, CascadeRowsLoader } from '../../ui/loaders';

interface LiveReadingRow {
    id: string;
    sensor_name: string;
    measurement_type: string;
    value: number | null;
    unit: string;
    time: string;
}

const columns = [
    { field: 'sensor_name', headerName: 'Sensor Name', flex: 1, minWidth: 150, compact: 'title' },
    { field: 'measurement_type', headerName: 'Measurement', flex: 1, minWidth: 120, compact: 'meta' },
    {
        field: 'value',
        headerName: 'Value',
        flex: 1,
        type: 'number',
        minWidth: 90,
        compact: 'meta',
        valueFormatter: (value: number | null, row: LiveReadingRow) => {
            if (value == null) return '—';
            return `${value}${row.unit ? ` ${row.unit}` : ''}`;
        },
    },
    {
        field: 'time',
        headerName: 'Time',
        flex: 1,
        minWidth: 200,
        compact: 'hidden',
        valueFormatter: (value: string) => new Date(value).toLocaleString(),
    },
] as const satisfies readonly DataTableColumn<LiveReadingRow>[];

export default function LiveReadingsTableWidget(_props: WidgetProps) {
    const reportUpdate = useReportWidgetUpdate();
    const readings = useCurrentReadings({ onDataUpdate: reportUpdate });
    const { loaded } = useSensorContext();
    useWidgetStateReport(useCurrentReadingsReady() ? 'populated' : 'loading');

    const rows: LiveReadingRow[] = Object.keys(readings)
        .sort((a, b) => a.localeCompare(b))
        .flatMap((sensor) =>
            Object.values(readings[sensor]).map((reading) => ({
                id: `${sensor}:${reading.measurement_type}`,
                sensor_name: reading.sensor_name,
                measurement_type: reading.measurement_type,
                value: reading.numeric_value,
                unit: reading.unit,
                time: reading.time,
            })),
        );

    return (
        <WidgetSwap loading={!loaded} loader={<CascadeRowsLoader />}>
            {rows.length === 0 ? (
                <EmptyState
                    icon={<ThermostatOutlinedIcon fontSize="large" />}
                    title="No live temperature data"
                    description="Add and enable sensors to see live readings here."
                    actionLabel="Go to Sensors"
                    actionHref="/sensors-overview"
                />
            ) : (
                <DataTable rows={rows} columns={columns} />
            )}
        </WidgetSwap>
    );
}
