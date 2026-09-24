import {DataGrid, type GridColDef, type GridRowParams} from "@mui/x-data-grid";
import type { AlertRule } from "../gen/aliases";
import EmptyState from '../ui/EmptyState';
import NotificationsNoneOutlinedIcon from "@mui/icons-material/NotificationsNoneOutlined";

interface AlertRuleDataGridProps {
  handleRowClick?: (params: GridRowParams, event: React.MouseEvent) => void;
  alertRules: AlertRule[];
  onCreateClick?: () => void;
}

export default function AlertRuleDataGrid({handleRowClick, alertRules, onCreateClick}: AlertRuleDataGridProps) {
  const safeRules = alertRules ?? [];

  const columns: GridColDef[] = [
    { field: 'SensorName', headerName: 'Sensor', flex: 1 },
    { field: 'MeasurementType', headerName: 'Measurement', width: 130 },
    { field: 'AlertType', headerName: 'Alert Type', width: 150 },
    { field: 'HighThreshold', headerName: 'High', width: 80 },
    { field: 'LowThreshold', headerName: 'Low', width: 80 },
    { field: 'TriggerStatus', headerName: 'Status', width: 100 },
    { field: 'RateLimitDisplay', headerName: 'Rate Limit', width: 130 },
    { field: 'Enabled', headerName: 'Enabled', width: 80 },
    { field: 'LastAlertSentAt', headerName: 'Last Alert Sent', width: 180 },
  ];

  const formatRateLimit = (seconds: number): string => {
    if (seconds === 0) return 'None';
    if (seconds < 60) return `${seconds}s`;
    if (seconds < 3600) return `${Math.round(seconds / 60)}m`;
    return `${Math.round(seconds / 3600)}h`;
  };

  const rows = safeRules.map(r => ({
    id: r.ID,
    ...r,
    HighThreshold: r.HighThreshold ?? '-',
    LowThreshold: r.LowThreshold ?? '-',
    TriggerStatus: r.TriggerStatus || '-',
    RateLimitDisplay: formatRateLimit(r.RateLimitSeconds),
    Enabled: r.Enabled ? 'Yes' : 'No',
    LastAlertSentAt: r.LastAlertSentAt ? new Date(r.LastAlertSentAt).toLocaleString() : 'Never',
  }));

  if (safeRules.length === 0) {
    return (
      <EmptyState
        icon={<NotificationsNoneOutlinedIcon sx={{ fontSize: 48 }} />}
        title="No alert rules configured"
        description="Create an alert rule to get notified when sensor readings go out of range."
        actionLabel={onCreateClick ? "Create Alert Rule" : undefined}
        onAction={onCreateClick}
        size="lg"
      />
    );
  }

  return (
    <DataGrid
      rows={rows}
      columns={columns}
      pageSizeOptions={[5, 10, 25]}
      initialState={{ pagination: { paginationModel: { pageSize: 10 } } }}
      onRowClick={handleRowClick}
    />
  )
}