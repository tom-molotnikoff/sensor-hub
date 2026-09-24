import { useState } from 'react';
import {
    Button, IconButton, MenuItem, Select, Tooltip, Typography,
    Dialog, DialogTitle, DialogContent, DialogActions, DialogContentText, TextField,
} from '@mui/material';
import EditIcon from '@mui/icons-material/Edit';
import LockIcon from '@mui/icons-material/Lock';
import SaveIcon from '@mui/icons-material/Save';
import AddIcon from '@mui/icons-material/Add';
import DeleteIcon from '@mui/icons-material/Delete';
import { useDashboard } from './DashboardContext';
import ActionBar from '../ui/ActionBar';
import { useAuth } from '../providers/AuthContext';
import { hasPerm } from '../tools/Utils';

interface DashboardToolbarProps {
    onAddWidget: () => void;
}

export default function DashboardToolbar({ onAddWidget }: DashboardToolbarProps) {
    const { user } = useAuth();
    const {
        dashboards, activeDashboard, isEditing,
        setIsEditing, setActiveDashboard, saveDashboard,
        createDashboard, deleteDashboard,
    } = useDashboard();

    const [showCreate, setShowCreate] = useState(false);
    const [showDelete, setShowDelete] = useState(false);
    const [newName, setNewName] = useState('');
    const canManage = hasPerm(user, 'manage_dashboards');

    const handleCreate = async () => {
        if (!newName.trim()) return;
        await createDashboard({ name: newName.trim(), config: { widgets: [] } });
        setNewName('');
        setShowCreate(false);
    };

    const handleDelete = async () => {
        if (!activeDashboard) return;
        await deleteDashboard(activeDashboard.id);
        setShowDelete(false);
    };

    return (
        <>
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
                trailing={canManage && (
                    <>
                        <Tooltip title="New dashboard">
                            <Button size="small" variant="outlined" onClick={() => setShowCreate(true)}>
                                New Dashboard
                            </Button>
                        </Tooltip>

                        {activeDashboard && (
                            <Tooltip title="Delete dashboard">
                                <IconButton size="small" color="error" onClick={() => setShowDelete(true)}>
                                    <DeleteIcon />
                                </IconButton>
                            </Tooltip>
                        )}
                    </>
                )}
            >
                {canManage && (
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
                )}

                {!canManage && (
                    <Typography variant="body2" sx={{ color: 'text.secondary' }}>
                        View only
                    </Typography>
                )}
            </ActionBar>
            <Dialog open={showCreate} onClose={() => setShowCreate(false)} maxWidth="xs" fullWidth>
                <DialogTitle>New Dashboard</DialogTitle>
                <DialogContent>
                    <TextField
                        autoFocus fullWidth margin="dense" label="Dashboard Name"
                        value={newName} onChange={(e) => setNewName(e.target.value)}
                        onKeyDown={(e) => e.key === 'Enter' && handleCreate()}
                    />
                </DialogContent>
                <DialogActions>
                    <Button onClick={() => setShowCreate(false)}>Cancel</Button>
                    <Button variant="contained" onClick={handleCreate} disabled={!newName.trim()}>Create</Button>
                </DialogActions>
            </Dialog>
            <Dialog open={showDelete} onClose={() => setShowDelete(false)} maxWidth="xs" fullWidth>
                <DialogTitle>Delete Dashboard</DialogTitle>
                <DialogContent>
                    <DialogContentText>
                        Are you sure you want to delete "{activeDashboard?.name}"? This action cannot be undone.
                    </DialogContentText>
                </DialogContent>
                <DialogActions>
                    <Button onClick={() => setShowDelete(false)}>Cancel</Button>
                    <Button variant="contained" color="error" onClick={handleDelete}>Delete</Button>
                </DialogActions>
            </Dialog>
        </>
    );
}
