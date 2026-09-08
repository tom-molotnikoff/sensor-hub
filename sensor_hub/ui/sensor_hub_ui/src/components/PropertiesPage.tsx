import { useState } from 'react';
import { Alert, Button, CircularProgress, Divider, Paper, Snackbar, Stack, Typography } from '@mui/material';
import { apiClient } from '../gen/client';
import { useProperties } from '../hooks/useProperties';
import { usePropertyDefinitions } from '../hooks/usePropertyDefinitions';
import { usePropertyEdits } from '../hooks/usePropertyEdits';
import { useAuth } from '../providers/AuthContext';
import { hasPerm } from '../tools/Utils';
import PropertyField from './PropertyField';
import { TypographyH2 } from '../tools/Typography.tsx';

export default function PropertiesPage() {
  const serverValues = useProperties();
  const { definitions } = usePropertyDefinitions();
  const { user } = useAuth();
  const canManage = !!user && hasPerm(user, 'manage_properties');
  const { edits, collisions, modifiedCount, edit, discard, discardAll, markSubmitted } =
    usePropertyEdits(serverValues);
  const [error, setError] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);
  const [saved, setSaved] = useState(false);

  if (!definitions) return null;

  const handleSave = async () => {
    setSaving(true);
    setError(null);
    setSaved(false);
    try {
      const payload: Record<string, string> = {};
      for (const definition of definitions.definitions) {
        if (definition.readOnly) continue;
        const value = edits[definition.key] ?? serverValues[definition.key];
        if (value !== undefined) payload[definition.key] = value;
      }
      await apiClient.PATCH('/properties', { body: payload as never });
      markSubmitted();
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
              {modifiedCount > 0 && (
                <>
                  <Typography variant="body2" color="text.secondary">
                    {modifiedCount === 1 ? '1 unsaved change' : `${modifiedCount} unsaved changes`}
                  </Typography>
                  <Button variant="text" color="inherit" onClick={discardAll} disabled={saving}>
                    Discard
                  </Button>
                </>
              )}
              <Button variant="contained" color="primary" onClick={handleSave} disabled={modifiedCount === 0 || saving}>
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
              editedValue={edits[definition.key]}
              collided={collisions.has(definition.key)}
              onChange={(value) => edit(definition.key, value)}
              onUndo={() => discard(definition.key)}
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
