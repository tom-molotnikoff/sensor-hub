import { useEffect, useMemo, useRef, useState } from 'react';
import { Alert, Button, CircularProgress, Snackbar, Typography } from '@mui/material';
import { apiClient } from '../gen/client';
import type { PropertyDefinition } from '../gen/aliases';
import { useProperties } from '../hooks/useProperties';
import { usePropertyDefinitions } from '../hooks/usePropertyDefinitions';
import { usePropertyEdits } from '../hooks/usePropertyEdits';
import { useMeasuredHeight } from '../hooks/useMeasuredHeight';
import { useScrollSpy } from '../hooks/useScrollSpy';
import { useAuth } from '../providers/AuthContext';
import { hasPerm } from '../tools/Utils';
import AnchorStack from '../ui/AnchorStack';
import Inline from '../ui/Inline';
import PageGrid from '../ui/PageGrid';
import Stack from '../ui/Stack';
import { StickyBar } from '../ui/Sticky';
import PropertyField from './PropertyField';
import PropertyGroupSection from './PropertyGroupSection';
import PropertySearchRail from './PropertySearchRail';
import { landingOffset, railOffset } from './propertyLayout';
import { buildSections } from './propertySections';
import { asRejection, propertyErrors } from './propertyValidation';
import type { PropertyRejection } from './propertyValidation';

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
  const canManage = !!user && hasPerm(user, 'manage_properties');
  const { edits, collisions, modified, edit, discard, discardAll, markSubmitted } =
    usePropertyEdits(serverValues);
  const [search, setSearch] = useState('');
  const [rejection, setRejection] = useState<PropertyRejection | null>(null);
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

  const allDefinitions = useMemo(
    () => allSections.flatMap((section) => section.rows.map((row) => row.definition)),
    [allSections],
  );

  const errors = useMemo(
    () => propertyErrors(allDefinitions, edits, rejection),
    [allDefinitions, edits, rejection],
  );

  const hiddenErrorCount = useMemo(() => {
    const rendered = new Set(
      sections.flatMap((section) => section.rows.map((row) => row.definition.key)),
    );
    return [...errors.fields.keys()].filter((key) => !rendered.has(key)).length;
  }, [sections, errors]);

  const { measuredRef: headerRef, height: headerHeight } = useMeasuredHeight();
  const landingLine = landingOffset(headerHeight);

  const currentGroupId = useScrollSpy(
    sections.map((section) => section.group.id),
    landingLine,
  );

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
    errorCount: groupRows.filter((row) => errors.fields.has(row.definition.key)).length,
  }));

  const forgetRejection = (key: string) =>
    setRejection((current) => (current?.key === key ? null : current));

  const handleEdit = (key: string, value: string) => {
    forgetRejection(key);
    edit(key, value);
  };

  const handleDiscard = (key: string) => {
    forgetRejection(key);
    discard(key);
  };

  const handleDiscardAll = () => {
    setRejection(null);
    discardAll();
  };

  const handleSave = async () => {
    setSaving(true);
    setRejection(null);
    setSaved(false);
    try {
      const payload: Record<string, string> = {};
      for (const { definition } of allSections.flatMap((section) => section.rows)) {
        if (definition.readOnly) continue;
        const value = edits[definition.key] ?? serverValues[definition.key];
        if (value !== undefined) payload[definition.key] = value;
      }
      const submitted = edits;
      const { error: apiError } = await apiClient.PATCH('/properties', { body: payload as never });
      if (apiError) {
        setRejection(asRejection(apiError));
        return;
      }
      markSubmitted(submitted);
      setSaved(true);
    } catch (e: unknown) {
      setRejection(asRejection(e));
    } finally {
      setSaving(false);
    }
  };

  return (
    <>
      <Stack>
        <StickyBar
          ref={headerRef}
          title="Properties"
          actions={
            canManage && (
              <Inline>
                {saving && <CircularProgress size={20} />}
                {modified.size > 0 && (
                  <>
                    <Typography variant="body2" color="text.secondary">
                      {modified.size === 1 ? '1 unsaved change' : `${modified.size} unsaved changes`}
                    </Typography>
                    <Button variant="text" color="inherit" onClick={handleDiscardAll} disabled={saving}>
                      Discard
                    </Button>
                  </>
                )}
                <Button
                  variant="contained"
                  color="primary"
                  onClick={handleSave}
                  disabled={modified.size === 0 || saving || errors.fields.size > 0}
                >
                  Save changes
                </Button>
              </Inline>
            )
          }
        />

        {definitionsError && (
          <Alert severity="warning">
            Property descriptions and typed controls are unavailable. Every property is editable as text.
          </Alert>
        )}

        {hiddenErrorCount > 0 && (
          <Alert severity="warning">
            {hiddenErrorCount === 1
              ? '1 field hidden by the search has an error. Clear the search to correct it.'
              : `${hiddenErrorCount} fields hidden by the search have errors. Clear the search to correct them.`}
          </Alert>
        )}

        {errors.page && <Alert severity="error">{errors.page}</Alert>}

        {loading ? (
          <Inline>
            <CircularProgress size={20} />
            <Typography color="text.secondary">Loading properties…</Typography>
          </Inline>
        ) : (
          <PageGrid equalHeight>
            <PageGrid.Item span={{ wide: 3 }}>
              <PropertySearchRail
                groups={railGroups}
                currentGroupId={currentGroupId}
                search={search}
                onSearchChange={setSearch}
                stickyOffset={railOffset(headerHeight)}
              />
            </PageGrid.Item>
            <PageGrid.Item span={{ wide: 9 }}>
              <AnchorStack landingOffset={landingLine}>
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
                        error={errors.fields.get(definition.key)}
                        onChange={(value) => handleEdit(definition.key, value)}
                        onUndo={() => handleDiscard(definition.key)}
                        disabled={!canManage}
                      />
                    ))}
                  </PropertyGroupSection>
                ))}
              </AnchorStack>
            </PageGrid.Item>
          </PageGrid>
        )}
      </Stack>
      <Snackbar
        open={saved}
        autoHideDuration={2000}
        onClose={() => setSaved(false)}
        anchorOrigin={{ vertical: 'bottom', horizontal: 'center' }}
      >
        <Alert severity="success">Properties updated successfully</Alert>
      </Snackbar>
    </>
  );
}
