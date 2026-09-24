import { useEffect, useState } from 'react';
import { List, ListItem, ListItemButton, ListItemText, Switch, Typography, Snackbar, Alert, CircularProgress } from '@mui/material';
import { apiClient } from '../gen/client';
import { useAuth } from '../providers/AuthContext';
import { hasPerm } from '../tools/Utils';
import { logger } from '../tools/logger';
import Card from '../ui/Card';
import Inline from '../ui/Inline';
import PageGrid from '../ui/PageGrid';
import Stack from '../ui/Stack';

type Role = { id: number; name: string };
type Permission = { id: number; name: string; description: string };

export default function RolePermissionsCard() {
  const [roles, setRoles] = useState<Role[]>([]);
  const [permissions, setPermissions] = useState<Permission[]>([]);
  const [selectedRole, setSelectedRole] = useState<Role | null>(null);
  const [rolePermissions, setRolePermissions] = useState<number[]>([]);
  const [loading, setLoading] = useState(true);
  const [toggling, setToggling] = useState<number[]>([]);
  const [snack, setSnack] = useState<{ open: boolean; message: string; severity: 'success' | 'error' }>({ open: false, message: '', severity: 'success' });
  const { user } = useAuth();

  const load = () =>
    apiClient.GET('/roles')
      .then(async ({ data: r }) => {
        setRoles(r ?? []);
        const { data: p } = await apiClient.GET('/roles/permissions');
        setPermissions((p as Permission[] | null) ?? []);
      })
      .catch((e) => logger.error(e))
      .finally(() => setLoading(false));

  const loadRolePerms = async (roleId: number) => {
    try {
      const { data: rp } = await apiClient.GET('/roles/{id}/permissions', { params: { path: { id: roleId } } });
      setRolePermissions(((rp as Permission[] | null) ?? []).map(x => x.id));
    } catch (e) { logger.error(e); setRolePermissions([]); }
  };

  useEffect(() => { void load(); }, []);

  const onRoleSelect = (r: Role) => { setSelectedRole(r); loadRolePerms(r.id); };

  const togglePermission = async (permId: number) => {
    if (!selectedRole) return;
    const has = rolePermissions.includes(permId);
    setToggling(t => [...t, permId]);
    try {
      if (has) {
        await apiClient.DELETE('/roles/{id}/permissions/{pid}', { params: { path: { id: selectedRole.id, pid: permId } } });
        setSnack({ open: true, message: 'Permission removed', severity: 'success' });
      } else {
        await apiClient.POST('/roles/{id}/permissions', { params: { path: { id: selectedRole.id } }, body: { permission_id: permId } as never });
        setSnack({ open: true, message: 'Permission added', severity: 'success' });
      }
      await loadRolePerms(selectedRole.id);
    } catch (e) {
      logger.error(e);
      setSnack({ open: true, message: 'Failed to update permission', severity: 'error' });
    } finally {
      setToggling(t => t.filter(id => id !== permId));
    }
  };

  if (!user || !(hasPerm(user, "manage_roles") || hasPerm(user, "view_roles"))) return null;

  const disabled = !hasPerm(user, "manage_roles");

  return (
    <>
      <PageGrid equalHeight>
        <PageGrid.Item span={{ wide: 4 }}>
          <Card title="Roles">
            {loading ? (
              <Inline><CircularProgress /></Inline>
            ) : roles.length === 0 ? (
              <Typography variant="body2">No roles found.</Typography>
            ) : (
              <List disablePadding>
                {roles.map(r => (
                  <ListItemButton key={r.id} selected={selectedRole?.id === r.id} onClick={() => onRoleSelect(r)}>
                    <ListItemText primary={r.name} />
                  </ListItemButton>
                ))}
              </List>
            )}
          </Card>
        </PageGrid.Item>

        <PageGrid.Item span={{ wide: 8 }}>
          <Card title="Permissions">
            <Stack>
              <Typography variant="body2" sx={{ color: "text.secondary" }}>
                {selectedRole
                  ? `Editing: ${selectedRole.name}`
                  : 'Select a role to view and modify its permissions.'}
              </Typography>
              {selectedRole && permissions.length === 0 && <Typography variant="body2">No permissions defined.</Typography>}
              {selectedRole && permissions.length > 0 && (
                <List disablePadding>
                  {permissions.map((p, index) => {
                    const busy = toggling.includes(p.id);
                    const checked = rolePermissions.includes(p.id);
                    return (
                      <ListItem
                        key={p.id}
                        disableGutters
                        divider={index < permissions.length - 1}
                        secondaryAction={busy ? <CircularProgress size={20} /> : (
                          <Switch
                            edge="end"
                            disabled={disabled}
                            checked={checked}
                            onChange={() => togglePermission(p.id)}
                            slotProps={{ input: { 'aria-label': p.name } }}
                          />
                        )}
                      >
                        <ListItemText primary={p.name} secondary={p.description} />
                      </ListItem>
                    );
                  })}
                </List>
              )}
            </Stack>
          </Card>
        </PageGrid.Item>
      </PageGrid>
      <Snackbar open={snack.open} autoHideDuration={3000} onClose={() => setSnack(s => ({ ...s, open: false }))}>
        <Alert severity={snack.severity} onClose={() => setSnack(s => ({ ...s, open: false }))}>{snack.message}</Alert>
      </Snackbar>
    </>
  );
}
