import {useState, useEffect} from "react";
import {
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
} from "@mui/material";
import type {User, RoleInfo} from "../gen/aliases";
import { apiClient } from "../gen/client";
import { logger } from '../tools/logger';

interface EditUserDialogProps {
  open: boolean;
  onClose: () => void;
  onSaved: () => Promise<void>;
  selectedUser: User | null;
}

export default function EditUserDialog({open, onClose, onSaved, selectedUser}: EditUserDialogProps) {
  const [role, setRole] = useState('user');
  const [availableRoles, setAvailableRoles] = useState<RoleInfo[]>([]);

  // Re-seed the role whenever the dialog opens for a user (adjust-during-render).
  const [prevOpen, setPrevOpen] = useState(open);
  const [prevUser, setPrevUser] = useState(selectedUser);
  if (prevOpen !== open || prevUser !== selectedUser) {
    setPrevOpen(open);
    setPrevUser(selectedUser);
    if (open) {
      setRole(selectedUser?.roles && selectedUser.roles.length > 0 ? selectedUser.roles[0] : 'user');
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
    try {
      await apiClient.POST('/users/{id}/roles', { params: { path: { id: selectedUser.id } }, body: { roles: [role] } as never });
      onClose();
      await onSaved();
    } catch (e) {
      logger.error('Failed to update user roles', e);
    }
  };

  return (
    <Dialog open={open} onClose={onClose}>
      <DialogTitle>Edit user</DialogTitle>
      <DialogContent>
        <TextField fullWidth label="Username" value={selectedUser?.username ?? ''} disabled sx={{mt: 1}}/>
        <FormControl fullWidth sx={{mt: 2}}>
          <InputLabel id="edit-role-select-label">Role</InputLabel>
          <Select labelId="edit-role-select-label" value={role} label="Role" onChange={(e) => setRole(e.target.value as string)}>
            {availableRoles.map(r => (<MenuItem key={r.name} value={r.name}>{r.name}</MenuItem>))}
          </Select>
        </FormControl>
      </DialogContent>
      <DialogActions>
        <Button onClick={onClose}>Cancel</Button>
        <Button variant="contained" onClick={handleSave}>Save</Button>
      </DialogActions>
    </Dialog>
  );
}
