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
import { apiClient } from "../gen/client";
import type { RoleInfo } from "../gen/aliases";
import { logger } from '../tools/logger';

interface CreateUserDialogProps {
  open: boolean;
  onClose: () => void;
  onCreated: () => Promise<void>;
}

export default function CreateUserDialog({open, onClose, onCreated}: CreateUserDialogProps) {
  const [username, setUsername] = useState('');
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [role, setRole] = useState('user');
  const [availableRoles, setAvailableRoles] = useState<RoleInfo[]>([]);

  useEffect(() => {
    if (!open) return;
    apiClient.GET('/roles').then(({ data: r }) => {
      const roles = r || [];
      setAvailableRoles(roles);
      if (roles.length > 0) {
        setRole(prev => (roles.find(x => x.name === prev) ? prev : roles[0].name));
      }
    }).catch(e => logger.error('Failed to load roles', e));
  }, [open]);

  const resetForm = () => {
    setUsername('');
    setEmail('');
    setPassword('');
    setRole('user');
  };

  const handleCreate = async () => {
    try {
      await apiClient.POST('/users', { body: { username, email, password, roles: [role] } });
      resetForm();
      onClose();
      await onCreated();
    } catch (e) {
      logger.error('Failed to create user', e);
    }
  };

  const handleCancel = () => {
    resetForm();
    onClose();
  };

  return (
    <Dialog open={open} onClose={handleCancel}>
      <DialogTitle>Create user</DialogTitle>
      <DialogContent>
        <TextField fullWidth label="Username" value={username} onChange={(e) => setUsername(e.target.value)} sx={{mt: 1}}/>
        <TextField fullWidth label="Email" value={email} onChange={(e) => setEmail(e.target.value)} sx={{mt: 1}}/>
        <TextField fullWidth label="Password" type="password" value={password} onChange={(e) => setPassword(e.target.value)} sx={{mt: 1}}/>
        <FormControl fullWidth sx={{mt: 1}}>
          <InputLabel id="role-select-label">Role</InputLabel>
          <Select labelId="role-select-label" value={role} label="Role" onChange={(e) => setRole(e.target.value as string)}>
            {availableRoles.map(r => (<MenuItem key={r.name} value={r.name}>{r.name}</MenuItem>))}
          </Select>
        </FormControl>
      </DialogContent>
      <DialogActions>
        <Button onClick={handleCancel}>Cancel</Button>
        <Button variant="contained" onClick={handleCreate}>Create</Button>
      </DialogActions>
    </Dialog>
  );
}
