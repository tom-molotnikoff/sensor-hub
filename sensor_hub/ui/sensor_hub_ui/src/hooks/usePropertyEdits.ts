import { useCallback, useEffect, useMemo, useRef, useState } from 'react';

interface EditState {
  values: Record<string, string>;
  bases: Record<string, string | undefined>;
}

const NOTHING_EDITED: EditState = { values: {}, bases: {} };

function without(state: EditState, keys: string[]): EditState {
  const values = { ...state.values };
  const bases = { ...state.bases };
  for (const key of keys) {
    delete values[key];
    delete bases[key];
  }
  return { values, bases };
}

export function usePropertyEdits(serverValues: Record<string, string>) {
  const [state, setState] = useState<EditState>(NOTHING_EDITED);
  const submitted = useRef<Record<string, string> | null>(null);

  useEffect(() => {
    const sent = submitted.current;
    if (!sent) return;
    submitted.current = null;
    setState((prev) =>
      without(prev, Object.keys(sent).filter((key) => prev.values[key] === sent[key])),
    );
  }, [serverValues]);

  const edit = useCallback(
    (key: string, value: string) => {
      setState((prev) => {
        if (value === serverValues[key]) return without(prev, [key]);
        const values = { ...prev.values, [key]: value };
        if (key in prev.values) return { values, bases: prev.bases };
        return { values, bases: { ...prev.bases, [key]: serverValues[key] } };
      });
    },
    [serverValues],
  );

  const discard = useCallback((key: string) => {
    setState((prev) => without(prev, [key]));
  }, []);

  const discardAll = useCallback(() => {
    setState(NOTHING_EDITED);
  }, []);

  const markSubmitted = useCallback((sent: Record<string, string>) => {
    submitted.current = sent;
  }, []);

  const modifiedKeys = useMemo(
    () => Object.keys(state.values).filter((key) => state.values[key] !== serverValues[key]),
    [state.values, serverValues],
  );

  const collisions = useMemo(
    () => new Set(modifiedKeys.filter((key) => state.bases[key] !== serverValues[key])),
    [modifiedKeys, state.bases, serverValues],
  );

  return {
    edits: state.values,
    collisions,
    modifiedCount: modifiedKeys.length,
    edit,
    discard,
    discardAll,
    markSubmitted,
  };
}
