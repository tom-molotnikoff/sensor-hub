import {useState, useEffect} from "react";
import {
  Alert,
  Button,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  TextField,
  FormControl,
  InputLabel,
  MenuItem,
  Select,
  FormControlLabel,
  Switch,
} from "@mui/material";
import type {User, RoleInfo} from "../gen/aliases";
import { apiClient } from "../gen/client";
import { unwrap } from '../api/unwrap';
import { useAuth } from '../providers/AuthContext';
import { logger } from '../tools/logger';
import Stack from '../ui/Stack';

interface EditUserDialogProps {
  open: boolean;
  onClose: () => void;
  onSaved: () => Promise<void>;
  selectedUser: User | null;
}

export default function EditUserDialog({open, onClose, onSaved, selectedUser}: EditUserDialogProps) {
  const [role, setRole] = useState('user');
  const [disabled, setDisabled] = useState(false);
  const [error, setError] = useState('');
  const [availableRoles, setAvailableRoles] = useState<RoleInfo[]>([]);
  const { user: currentUser } = useAuth();
  const isSelf = !!currentUser && currentUser.id === selectedUser?.id;

  // Re-seed the form whenever the dialog opens for a user (adjust-during-render).
  const [prevOpen, setPrevOpen] = useState(open);
  const [prevUser, setPrevUser] = useState(selectedUser);
  if (prevOpen !== open || prevUser !== selectedUser) {
    setPrevOpen(open);
    setPrevUser(selectedUser);
    if (open) {
      setRole(selectedUser?.roles && selectedUser.roles.length > 0 ? selectedUser.roles[0] : 'user');
      setDisabled(selectedUser?.disabled ?? false);
      setError('');
    }
  }

  useEffect(() => {
    if (!open) return;
    apiClient.GET('/roles').then(({ data: r }) => {
      setAvailableRoles(r || []);
    }).catch(e => logger.error('Failed to load roles', e));
  }, [open, selectedUser]);

  const handleSave = async () => {
    if (!selectedUser) return;
    setError('');
    try {
      if (disabled !== selectedUser.disabled) {
        await unwrap(apiClient.PUT('/users/{id}/disabled', { params: { path: { id: selectedUser.id } }, body: { disabled } }));
      }
      const { response } = await apiClient.POST('/users/{id}/roles', { params: { path: { id: selectedUser.id } }, body: { roles: [role] } });
      if (!response.ok) throw new Error(`${response.status} ${response.statusText}`);
      onClose();
      await onSaved();
    } catch (e) {
      logger.error('Failed to update user', e);
      setError(e instanceof Error ? e.message : 'Failed to update user');
    }
  };

  return (
    <Dialog open={open} onClose={onClose}>
      <DialogTitle>Edit user</DialogTitle>
      <DialogContent>
        <Stack>
          {error && <Alert severity="error">{error}</Alert>}
          <TextField fullWidth label="Username" value={selectedUser?.username ?? ''} disabled/>
          <FormControl fullWidth>
            <InputLabel id="edit-role-select-label">Role</InputLabel>
            <Select labelId="edit-role-select-label" value={role} label="Role" onChange={(e) => setRole(e.target.value as string)}>
              {availableRoles.map(r => (<MenuItem key={r.name} value={r.name}>{r.name}</MenuItem>))}
            </Select>
          </FormControl>
          <FormControlLabel
            control={<Switch checked={disabled} onChange={(e) => setDisabled(e.target.checked)} disabled={isSelf} />}
            label={isSelf ? 'Disabled (you cannot disable yourself)' : 'Disabled'}
          />
        </Stack>
      </DialogContent>
      <DialogActions>
        <Button onClick={onClose}>Cancel</Button>
        <Button variant="contained" onClick={handleSave}>Save</Button>
      </DialogActions>
    </Dialog>
  );
}
