import { createContext, useContext } from 'react';

// Two separate contexts prevent widgets from re-rendering when only the timestamp changes.
// Widgets read ReportContext (stable), only the badge reads ValueContext (changes on updates).
export const ReportContext = createContext<(date: Date) => void>(() => {});
export const ValueContext = createContext<Date | null>(null);

/** Returns the reportUpdate function. Safe to call outside a provider (returns no-op). */
export function useReportWidgetUpdate(): (date: Date) => void {
    return useContext(ReportContext);
}

/** Returns the last updated timestamp for the current widget. */
export function useWidgetLastUpdated(): Date | null {
    return useContext(ValueContext);
}
