import { useEffect, useMemo, useRef, useState } from 'react';
import { Alert, Box, Button, CircularProgress, Snackbar, Stack, Typography } from '@mui/material';
import { apiClient } from '../gen/client';
import type { PropertyDefinition } from '../gen/aliases';
import { useProperties } from '../hooks/useProperties';
import { usePropertyDefinitions } from '../hooks/usePropertyDefinitions';
import { usePropertyEdits } from '../hooks/usePropertyEdits';
import { useIsMobile } from '../hooks/useMobile';
import { useScrollSpy } from '../hooks/useScrollSpy';
import { useAuth } from '../providers/AuthContext';
import { hasPerm } from '../tools/Utils';
import PropertyField from './PropertyField';
import PropertyGroupSection from './PropertyGroupSection';
import PropertySearchRail from './PropertySearchRail';
import { buildSections } from './propertySections';
import { TypographyH2 } from '../tools/Typography.tsx';

function matchesSearch(definition: PropertyDefinition, term: string): boolean {
  if (term === '') return true;
  return [definition.key, definition.label, definition.description].some((text) =>
    text.toLowerCase().includes(term),
  );
}

export default function PropertiesPage() {
  const serverValues = useProperties();
  const { definitions, loading, error: definitionsError } = usePropertyDefinitions();
  const { user } = useAuth();
  const isMobile = useIsMobile();
  const canManage = !!user && hasPerm(user, 'manage_properties');
  const { edits, collisions, modified, edit, discard, discardAll, markSubmitted } =
    usePropertyEdits(serverValues);
  const [search, setSearch] = useState('');
  const [error, setError] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);
  const [saved, setSaved] = useState(false);

  const allSections = useMemo(
    () => buildSections(definitions, Object.keys(serverValues)),
    [definitions, serverValues],
  );

  const sections = useMemo(() => {
    const term = search.trim().toLowerCase();
    return allSections
      .map(({ group, rows }) => ({
        group,
        groupRows: rows,
        rows: rows.filter((row) => matchesSearch(row.definition, term)),
      }))
      .filter((section) => section.rows.length > 0);
  }, [allSections, search]);

  const currentGroupId = useScrollSpy(sections.map((section) => section.group.id));

  const landed = useRef(false);
  useEffect(() => {
    if (landed.current || loading || sections.length === 0) return;
    landed.current = true;
    const target = window.location.hash.slice(1);
    if (target === '') return;
    document.getElementById(target)?.scrollIntoView?.();
  }, [loading, sections.length]);

  const railGroups = sections.map(({ group, groupRows }) => ({
    id: group.id,
    label: group.label,
    editedCount: groupRows.filter((row) => modified.has(row.definition.key)).length,
  }));

  const handleSave = async () => {
    setSaving(true);
    setError(null);
    setSaved(false);
    try {
      const payload: Record<string, string> = {};
      for (const { definition } of allSections.flatMap((section) => section.rows)) {
        if (definition.readOnly) continue;
        const value = edits[definition.key] ?? serverValues[definition.key];
        if (value !== undefined) payload[definition.key] = value;
      }
      const submitted = edits;
      await apiClient.PATCH('/properties', { body: payload as never });
      markSubmitted(submitted);
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
      <Stack spacing={2} sx={{ width: '100%' }}>
        <Stack direction="row" sx={{ justifyContent: 'space-between', alignItems: 'center' }}>
          <TypographyH2>Properties</TypographyH2>
          {canManage && (
            <Stack direction="row" spacing={1} sx={{ alignItems: 'center' }}>
              {saving && <CircularProgress size={20} />}
              {modified.size > 0 && (
                <>
                  <Typography variant="body2" color="text.secondary">
                    {modified.size === 1 ? '1 unsaved change' : `${modified.size} unsaved changes`}
                  </Typography>
                  <Button variant="text" color="inherit" onClick={discardAll} disabled={saving}>
                    Discard
                  </Button>
                </>
              )}
              <Button variant="contained" color="primary" onClick={handleSave} disabled={modified.size === 0 || saving}>
                Save changes
              </Button>
            </Stack>
          )}
        </Stack>

        {definitionsError && (
          <Alert severity="warning">
            Property descriptions and typed controls are unavailable. Every property is editable as text.
          </Alert>
        )}

        {error && <Typography color="error">Error: {error}</Typography>}

        {loading ? (
          <Stack direction="row" spacing={1} sx={{ alignItems: 'center' }}>
            <CircularProgress size={20} />
            <Typography color="text.secondary">Loading properties…</Typography>
          </Stack>
        ) : (
          <Box
            data-testid="properties-layout"
            sx={{
              display: 'flex',
              flexDirection: isMobile ? 'column' : 'row',
              alignItems: 'flex-start',
              gap: 2,
            }}
          >
            <PropertySearchRail
              groups={railGroups}
              currentGroupId={currentGroupId}
              search={search}
              onSearchChange={setSearch}
            />
            <Stack spacing={2} sx={{ flex: '1 1 auto', minWidth: 0, width: '100%' }}>
              {sections.map(({ group, rows }) => (
                <PropertyGroupSection key={group.id} group={group}>
                  {rows.map(({ definition, described }) => (
                    <PropertyField
                      key={definition.key}
                      definition={definition}
                      described={described}
                      serverValue={serverValues[definition.key]}
                      editedValue={edits[definition.key]}
                      collided={collisions.has(definition.key)}
                      onChange={(value) => edit(definition.key, value)}
                      onUndo={() => discard(definition.key)}
                      disabled={!canManage}
                    />
                  ))}
                </PropertyGroupSection>
              ))}
            </Stack>
          </Box>
        )}
      </Stack>
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
