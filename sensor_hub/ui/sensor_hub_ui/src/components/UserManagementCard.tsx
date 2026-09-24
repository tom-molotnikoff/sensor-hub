import { useEffect, useMemo, useState } from 'react';
import { Button, Menu, MenuItem } from '@mui/material';
import { apiClient } from '../gen/client';
import type { User } from '../gen/aliases';
import { useAuth } from '../providers/AuthContext';
import { hasPerm } from '../tools/Utils';
import CreateUserDialog from './CreateUserDialog';
import EditUserDialog from './EditUserDialog';
import DeleteUserDialog from './DeleteUserDialog';
import { logger } from '../tools/logger';
import Card from '../ui/Card';
import DataTable from '../ui/DataTable';

export default function UserManagementCard() {
  const [users, setUsers] = useState<User[]>([]);
  const [menuAnchorEl, setMenuAnchorEl] = useState<null | HTMLElement>(null);
  const [selectedRow, setSelectedRow] = useState<User | null>(null);
  const [openCreateDialog, setOpenCreateDialog] = useState(false);
  const [openEditDialog, setOpenEditDialog] = useState(false);
  const [openDeleteDialog, setOpenDeleteDialog] = useState(false);
  const { user } = useAuth();

  const load = () =>
    apiClient.GET('/users')
      .then(({ data }) => setUsers(data ?? []))
      .catch((e) => logger.error(e));

  useEffect(() => { void load(); }, []);

  const handleRowClick = (row: User, anchor: HTMLElement) => {
    setSelectedRow(row);
    setMenuAnchorEl(anchor);
  };

  const closeMenu = () => { setMenuAnchorEl(null); };

  const handleForceChange = async () => {
    if (!selectedRow) return;
    closeMenu();
    await apiClient.PATCH('/users/{id}/must_change', { params: { path: { id: selectedRow.id } }, body: { must_change: true } as never });
    await load();
  };

  const rows = useMemo(() => users.map(u => ({ ...u, rolesDisplay: (u.roles || []).join(', ') })), [users]);
  const canManage = !!user && hasPerm(user, "manage_users");

  return (
    <>
      <Card
        title="Manage Users"
        actions={<Button variant="contained" onClick={() => setOpenCreateDialog(true)} disabled={!canManage}>Create user</Button>}
      >
        <DataTable
          rows={rows}
          onRowClick={canManage ? handleRowClick : undefined}
          columns={[
            { field: 'id', headerName: 'ID', width: 80, compact: 'hidden' },
            { field: 'username', headerName: 'Username', flex: 1, minWidth: 140, compact: 'title' },
            { field: 'email', headerName: 'Email', flex: 1, minWidth: 160, compact: 'meta' },
            { field: 'rolesDisplay', headerName: 'Roles', flex: 1, minWidth: 120, compact: 'meta' },
            { field: 'must_change_password', headerName: 'Must change password', width: 200, compact: 'hidden' },
          ]}
        />

        {canManage && (
          <Menu anchorEl={menuAnchorEl} open={Boolean(menuAnchorEl)} onClose={closeMenu}>
            <MenuItem onClick={() => { closeMenu(); setOpenEditDialog(true); }}>Edit</MenuItem>
            <MenuItem onClick={() => { closeMenu(); setOpenDeleteDialog(true); }}>Delete</MenuItem>
            <MenuItem onClick={handleForceChange}>Force change password</MenuItem>
          </Menu>
        )}
      </Card>
      <CreateUserDialog open={openCreateDialog} onClose={() => setOpenCreateDialog(false)} onCreated={load} />
      <EditUserDialog open={openEditDialog} onClose={() => setOpenEditDialog(false)} onSaved={load} selectedUser={selectedRow} />
      <DeleteUserDialog open={openDeleteDialog} onClose={() => setOpenDeleteDialog(false)} onDeleted={load} selectedUser={selectedRow} />
    </>
  );
}
