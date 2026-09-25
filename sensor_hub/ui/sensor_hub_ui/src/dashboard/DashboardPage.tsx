import { useState, type ReactNode } from 'react';
import { Button, Dialog, DialogTitle, DialogContent, DialogActions, DialogContentText, TextField } from '@mui/material';
import AddIcon from '@mui/icons-material/Add';
import DashboardIcon from '@mui/icons-material/Dashboard';
import Page from '../ui/Page';
import Stack from '../ui/Stack';
import { useDashboard } from './DashboardContext';
import { DashboardProvider } from './DashboardProvider';
import DashboardEngine from './DashboardEngine';
import DashboardSkeleton from './DashboardSkeleton';
import DashboardTitle from './DashboardTitle';
import DashboardToolbar, { DashboardHeaderActions, DashboardLock } from './DashboardToolbar';
import WidgetPickerDialog from './WidgetPickerDialog';
import WidgetConfigDialog from './WidgetConfigDialog';
import EmptyState from '../ui/EmptyState';
import { registerAllWidgets } from './widgets';
import { useAuth } from '../providers/AuthContext';
import { hasPerm } from '../tools/Utils';
import { useTier } from '../ui/tiers';

registerAllWidgets();

type DashboardDialog = 'create' | 'delete';

interface CreateDashboardDialogProps {
    open: boolean;
    name: string;
    onNameChange: (name: string) => void;
    onCreate: () => void;
    onClose: () => void;
}

function CreateDashboardDialog({ open, name, onNameChange, onCreate, onClose }: CreateDashboardDialogProps) {
    return (
        <Dialog open={open} onClose={onClose} maxWidth="xs" fullWidth>
            <DialogTitle>New Dashboard</DialogTitle>
            <DialogContent>
                <TextField
                    autoFocus fullWidth margin="dense" label="Dashboard Name"
                    value={name} onChange={(e) => onNameChange(e.target.value)}
                    onKeyDown={(e) => e.key === 'Enter' && onCreate()}
                />
            </DialogContent>
            <DialogActions>
                <Button onClick={onClose}>Cancel</Button>
                <Button variant="contained" onClick={onCreate} disabled={!name.trim()}>Create</Button>
            </DialogActions>
        </Dialog>
    );
}

interface DeleteDashboardDialogProps {
    open: boolean;
    name?: string;
    onDelete: () => void;
    onClose: () => void;
}

function DeleteDashboardDialog({ open, name, onDelete, onClose }: DeleteDashboardDialogProps) {
    return (
        <Dialog open={open} onClose={onClose} maxWidth="xs" fullWidth>
            <DialogTitle>Delete Dashboard</DialogTitle>
            <DialogContent>
                <DialogContentText>
                    Are you sure you want to delete "{name}"? This action cannot be undone.
                </DialogContentText>
            </DialogContent>
            <DialogActions>
                <Button onClick={onClose}>Cancel</Button>
                <Button variant="contained" color="error" onClick={onDelete}>Delete</Button>
            </DialogActions>
        </Dialog>
    );
}

interface DashboardContentProps {
    canManage: boolean;
    toolbar: ReactNode;
    onAddWidget: () => void;
    onConfigureWidget: (id: string) => void;
    onNewDashboard: () => void;
}

function DashboardContent({ canManage, toolbar, onAddWidget, onConfigureWidget, onNewDashboard }: DashboardContentProps) {
    const { config, isEditing, loading, updateWidgets, removeWidget, activeDashboard } = useDashboard();

    if (loading) return <DashboardSkeleton />;

    if (!activeDashboard) {
        return (
            <EmptyState
                icon={<DashboardIcon fontSize="large" />}
                title="No dashboards yet"
                description={canManage ? 'Create your first dashboard to get started.' : 'No dashboards are available.'}
                actionLabel={canManage ? 'Create Dashboard' : undefined}
                onAction={canManage ? onNewDashboard : undefined}
            />
        );
    }

    if (config.widgets.length === 0 && !isEditing) {
        return (
            <Stack>
                {toolbar}
                <EmptyState
                    icon={<DashboardIcon fontSize="large" />}
                    title="Empty dashboard"
                    description={canManage ? 'Click Edit then Add Widget to populate this dashboard.' : 'This dashboard has no widgets yet.'}
                />
            </Stack>
        );
    }

    return (
        <Stack>
            {toolbar}

            {config.widgets.length === 0 && isEditing ? (
                <EmptyState
                    icon={<AddIcon fontSize="large" />}
                    title="Empty dashboard"
                    actionLabel="Add your first widget"
                    onAction={onAddWidget}
                />
            ) : (
                <DashboardEngine
                    config={config}
                    isEditing={isEditing}
                    onLayoutChange={updateWidgets}
                    onRemoveWidget={removeWidget}
                    onConfigureWidget={onConfigureWidget}
                    onAddWidget={onAddWidget}
                />
            )}
        </Stack>
    );
}

function DashboardPageInner() {
    const { user } = useAuth();
    const wide = useTier() === 'wide';
    const { loading, activeDashboard, createDashboard, deleteDashboard } = useDashboard();
    const [pickerOpen, setPickerOpen] = useState(false);
    const [configWidgetId, setConfigWidgetId] = useState<string | null>(null);
    const [dialog, setDialog] = useState<DashboardDialog | null>(null);
    const [newName, setNewName] = useState('');

    const canManage = hasPerm(user, 'manage_dashboards');
    const showControls = !loading && activeDashboard !== null;
    const openPicker = () => setPickerOpen(true);
    const openCreate = () => setDialog('create');
    const openDelete = () => setDialog('delete');
    const closeDialog = () => setDialog(null);

    const handleCreate = async () => {
        if (!newName.trim()) return;
        await createDashboard({ name: newName.trim(), config: { widgets: [] } });
        setNewName('');
        closeDialog();
    };

    const handleDelete = async () => {
        if (!activeDashboard) return;
        await deleteDashboard(activeDashboard.id);
        closeDialog();
    };

    return (
        <Page
            title="Dashboards"
            titleElement={showControls ? <DashboardTitle /> : undefined}
            beforeTitle={showControls && canManage && <DashboardLock edge="start" />}
            actions={showControls && <DashboardHeaderActions onAddWidget={openPicker} onNewDashboard={openCreate} onDeleteDashboard={openDelete} />}
            loading={user === undefined}
        >
            <DashboardContent
                canManage={canManage}
                toolbar={!wide && <DashboardToolbar onAddWidget={openPicker} onNewDashboard={openCreate} onDeleteDashboard={openDelete} />}
                onAddWidget={openPicker}
                onConfigureWidget={setConfigWidgetId}
                onNewDashboard={openCreate}
            />

            <WidgetPickerDialog open={pickerOpen} onClose={() => setPickerOpen(false)} />
            <WidgetConfigDialog open={!!configWidgetId} widgetId={configWidgetId} onClose={() => setConfigWidgetId(null)} />
            <CreateDashboardDialog
                open={dialog === 'create'}
                name={newName}
                onNameChange={setNewName}
                onCreate={handleCreate}
                onClose={closeDialog}
            />
            <DeleteDashboardDialog
                open={dialog === 'delete'}
                name={activeDashboard?.name}
                onDelete={handleDelete}
                onClose={closeDialog}
            />
        </Page>
    );
}

export default function DashboardPage() {
    return (
        <DashboardProvider>
            <DashboardPageInner />
        </DashboardProvider>
    );
}
