import { useState } from 'react';
import { Box, Button, Dialog, DialogTitle, DialogContent, DialogActions, TextField } from '@mui/material';
import AddIcon from '@mui/icons-material/Add';
import DashboardIcon from '@mui/icons-material/Dashboard';
import PageContainer from '../tools/PageContainer';
import { useDashboard } from './DashboardContext';
import { DashboardProvider } from './DashboardProvider';
import DashboardEngine from './DashboardEngine';
import DashboardSkeleton from './DashboardSkeleton';
import DashboardToolbar from './DashboardToolbar';
import WidgetPickerDialog from './WidgetPickerDialog';
import WidgetConfigDialog from './WidgetConfigDialog';
import EmptyState from '../components/EmptyState';
import { registerAllWidgets } from './widgets';
import { useAuth } from '../providers/AuthContext';
import { hasPerm } from '../tools/Utils';
import { DEFAULT_BREAKPOINTS } from './constants';

registerAllWidgets();

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

function DashboardPageInner() {
    const { user } = useAuth();
    const { config, isEditing, loading, updateWidgets, removeWidget, activeDashboard, createDashboard } = useDashboard();
    const [pickerOpen, setPickerOpen] = useState(false);
    const [configWidgetId, setConfigWidgetId] = useState<string | null>(null);
    const [showCreate, setShowCreate] = useState(false);
    const [newName, setNewName] = useState('');

    if (loading) return <DashboardSkeleton />;

    const canManage = hasPerm(user, 'manage_dashboards');

    const handleCreate = async () => {
        if (!newName.trim()) return;
        await createDashboard({ name: newName.trim(), config: { widgets: [], breakpoints: DEFAULT_BREAKPOINTS } });
        setNewName('');
        setShowCreate(false);
    };

    if (!activeDashboard) {
        return (
            <>
                <EmptyState
                    icon={<DashboardIcon sx={{ fontSize: 48 }} />}
                    title="No dashboards yet"
                    description={canManage ? 'Create your first dashboard to get started.' : 'No dashboards are available.'}
                    actionLabel={canManage ? 'Create Dashboard' : undefined}
                    onAction={canManage ? () => setShowCreate(true) : undefined}
                />
                <CreateDashboardDialog
                    open={showCreate}
                    name={newName}
                    onNameChange={setNewName}
                    onCreate={handleCreate}
                    onClose={() => setShowCreate(false)}
                />
            </>
        );
    }

    if (config.widgets.length === 0 && !isEditing) {
        return (
            <>
                <DashboardToolbar onAddWidget={() => setPickerOpen(true)} />
                <EmptyState
                    icon={<DashboardIcon sx={{ fontSize: 48 }} />}
                    title="Empty dashboard"
                    description={canManage ? 'Click Edit then Add Widget to populate this dashboard.' : 'This dashboard has no widgets yet.'}
                />
            </>
        );
    }

    return (
        <>
            <DashboardToolbar onAddWidget={() => setPickerOpen(true)} />

            {config.widgets.length === 0 && isEditing ? (
                <Box sx={{ display: 'flex', justifyContent: 'center', mt: 4 }}>
                    <Button variant="outlined" startIcon={<AddIcon />} onClick={() => setPickerOpen(true)}>
                        Add your first widget
                    </Button>
                </Box>
            ) : (
                <DashboardEngine
                    config={config}
                    isEditing={isEditing}
                    onLayoutChange={updateWidgets}
                    onRemoveWidget={removeWidget}
                    onConfigureWidget={(id) => setConfigWidgetId(id)}
                />
            )}

            <WidgetPickerDialog open={pickerOpen} onClose={() => setPickerOpen(false)} />
            <WidgetConfigDialog open={!!configWidgetId} widgetId={configWidgetId} onClose={() => setConfigWidgetId(null)} />
        </>
    );
}

export default function DashboardPage() {
    const { user } = useAuth();

    return (
        <PageContainer titleText="Dashboards" loading={user === undefined}>
            <DashboardProvider>
                <DashboardPageInner />
            </DashboardProvider>
        </PageContainer>
    );
}
