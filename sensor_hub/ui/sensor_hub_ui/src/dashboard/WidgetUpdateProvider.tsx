import { useCallback, useState, type ReactNode } from 'react';
import { ReportContext, ValueContext } from './WidgetUpdateContext';

export function WidgetUpdateProvider({ children }: { children: ReactNode }) {
    const [lastUpdated, setLastUpdated] = useState<Date | null>(null);
    const reportUpdate = useCallback((date: Date) => setLastUpdated(date), []);

    return (
        <ReportContext.Provider value={reportUpdate}>
            <ValueContext.Provider value={lastUpdated}>
                {children}
            </ValueContext.Provider>
        </ReportContext.Provider>
    );
}
