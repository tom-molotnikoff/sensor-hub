import { Button, IconButton, MenuItem, Select, Tooltip, Typography } from '@mui/material';
import EditIcon from '@mui/icons-material/Edit';
import LockIcon from '@mui/icons-material/Lock';
import SaveIcon from '@mui/icons-material/Save';
import AddIcon from '@mui/icons-material/Add';
import DeleteIcon from '@mui/icons-material/Delete';
import { useDashboard } from './DashboardContext';
import ActionBar from '../ui/ActionBar';
import { useAuth } from '../providers/AuthContext';
import { hasPerm } from '../tools/Utils';

interface DashboardEditControlsProps {
    onAddWidget: () => void;
}

export function DashboardEditControls({ onAddWidget }: DashboardEditControlsProps) {
    const { user } = useAuth();
    const { isEditing, setIsEditing, saveDashboard } = useDashboard();

    if (!hasPerm(user, 'manage_dashboards')) {
        return (
            <Typography variant="body2" sx={{ color: 'text.secondary' }}>
                View only
            </Typography>
        );
    }

    return (
        <>
            <Tooltip title={isEditing ? 'Lock dashboard' : 'Edit dashboard'}>
                <IconButton onClick={() => setIsEditing(!isEditing)} color={isEditing ? 'primary' : 'default'}>
                    {isEditing ? <EditIcon /> : <LockIcon />}
                </IconButton>
            </Tooltip>

            {isEditing && (
                <>
                    <Button startIcon={<SaveIcon />} variant="contained" size="small" onClick={saveDashboard}>
                        Save
                    </Button>
                    <Button startIcon={<AddIcon />} variant="outlined" size="small" onClick={onAddWidget}>
                        Add Widget
                    </Button>
                </>
            )}
        </>
    );
}

interface DashboardManageActionsProps {
    onNewDashboard: () => void;
    onDeleteDashboard: () => void;
}

export function DashboardManageActions({ onNewDashboard, onDeleteDashboard }: DashboardManageActionsProps) {
    const { user } = useAuth();
    const { activeDashboard } = useDashboard();

    if (!hasPerm(user, 'manage_dashboards')) return null;

    return (
        <>
            <Tooltip title="New dashboard">
                <Button size="small" variant="outlined" onClick={onNewDashboard}>
                    New Dashboard
                </Button>
            </Tooltip>

            {activeDashboard && (
                <Tooltip title="Delete dashboard">
                    <IconButton size="small" color="error" onClick={onDeleteDashboard}>
                        <DeleteIcon />
                    </IconButton>
                </Tooltip>
            )}
        </>
    );
}

type DashboardToolbarProps = DashboardEditControlsProps & DashboardManageActionsProps;

export default function DashboardToolbar({ onAddWidget, onNewDashboard, onDeleteDashboard }: DashboardToolbarProps) {
    const { user } = useAuth();
    const { dashboards, activeDashboard, setActiveDashboard } = useDashboard();

    return (
        <ActionBar
            picker={dashboards.length > 0 && (
                <Select
                    size="small"
                    value={activeDashboard?.id ?? ''}
                    onChange={(e) => {
                        const db = dashboards.find((d) => d.id === Number(e.target.value));
                        if (db) setActiveDashboard(db);
                    }}
                >
                    {dashboards.map((d) => (
                        <MenuItem key={d.id} value={d.id}>
                            {d.name}{d.is_default ? ' ★' : ''}
                        </MenuItem>
                    ))}
                </Select>
            )}
            trailing={hasPerm(user, 'manage_dashboards') && (
                <DashboardManageActions onNewDashboard={onNewDashboard} onDeleteDashboard={onDeleteDashboard} />
            )}
        >
            <DashboardEditControls onAddWidget={onAddWidget} />
        </ActionBar>
    );
}
