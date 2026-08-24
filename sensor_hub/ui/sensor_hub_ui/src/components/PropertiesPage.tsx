import { useState } from 'react';
import { Alert, Button, CircularProgress, Divider, Paper, Snackbar, Stack, Typography } from '@mui/material';
import { apiClient } from '../gen/client';
import { useProperties } from '../hooks/useProperties';
import { usePropertyDefinitions } from '../hooks/usePropertyDefinitions';
import { useAuth } from '../providers/AuthContext';
import { hasPerm } from '../tools/Utils';
import PropertyField from './PropertyField';
import { TypographyH2 } from '../tools/Typography.tsx';

export default function PropertiesPage() {
  const serverValues = useProperties();
  const { definitions } = usePropertyDefinitions();
  const { user } = useAuth();
  const canManage = !!user && hasPerm(user, 'manage_properties');
  const [editedValues, setEditedValues] = useState<Record<string, string>>({});
  const [error, setError] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);
  const [saved, setSaved] = useState(false);

  if (!definitions) return null;

  const isDirty = Object.entries(editedValues).some(([key, value]) => serverValues[key] !== value);

  const handleSave = async () => {
    setSaving(true);
    setError(null);
    setSaved(false);
    try {
      const payload: Record<string, string> = {};
      for (const definition of definitions.definitions) {
        if (definition.readOnly) continue;
        const value = editedValues[definition.key] ?? serverValues[definition.key];
        if (value !== undefined) payload[definition.key] = value;
      }
      await apiClient.PATCH('/properties', { body: payload as never });
      setSaved(true);
    } catch (e: unknown) {
      let msg: string;
      if (e && typeof e === 'object' && 'message' in e && typeof (e as { message?: unknown }).message === 'string') {
        msg = (e as { message: string }).message;
      } else {
        try { msg = JSON.stringify(e); } catch { msg = String(e); }
      }
      setError(msg);
    } finally {
      setSaving(false);
    }
  };

  return (
    <>
      <Paper sx={{ padding: 2, width: '100%' }}>
        <Stack direction="row" sx={{ justifyContent: 'space-between', alignItems: 'center', mb: 2 }}>
          <TypographyH2>Properties</TypographyH2>
          {canManage && (
            <Stack direction="row" spacing={1} sx={{ alignItems: 'center' }}>
              {saving && <CircularProgress size={20} />}
              <Button variant="contained" color="primary" onClick={handleSave} disabled={!isDirty || saving}>
                Save changes
              </Button>
            </Stack>
          )}
        </Stack>

        {error && <Typography color="error">Error: {error}</Typography>}

        <Stack divider={<Divider />}>
          {definitions.definitions.map((definition) => (
            <PropertyField
              key={definition.key}
              definition={definition}
              serverValue={serverValues[definition.key]}
              editedValue={editedValues[definition.key]}
              onChange={(value) => setEditedValues((prev) => ({ ...prev, [definition.key]: value }))}
              onUndo={() =>
                setEditedValues((prev) => {
                  const next = { ...prev };
                  delete next[definition.key];
                  return next;
                })
              }
              disabled={!canManage}
            />
          ))}
        </Stack>
      </Paper>
      <Snackbar
        open={saved}
        autoHideDuration={2000}
        onClose={() => setSaved(false)}
        anchorOrigin={{ vertical: 'bottom', horizontal: 'center' }}
      >
        <Alert severity="success" sx={{ width: '100%' }}>Properties updated successfully</Alert>
      </Snackbar>
    </>
  );
}
