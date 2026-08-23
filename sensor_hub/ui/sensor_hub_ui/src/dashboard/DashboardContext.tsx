import { createContext, useContext } from 'react';
import type { Dashboard, DashboardConfig, DashboardWidget, CreateDashboardRequest } from '../gen/aliases';

interface DashboardContextValue {
    dashboards: Dashboard[];
    activeDashboard: Dashboard | null;
    config: DashboardConfig;
    isEditing: boolean;
    loading: boolean;
    setIsEditing: (editing: boolean) => void;
    setActiveDashboard: (dashboard: Dashboard) => void;
    updateWidgets: (widgets: DashboardWidget[]) => void;
    addWidget: (widget: DashboardWidget) => void;
    removeWidget: (id: string) => void;
    updateWidgetConfig: (id: string, config: Record<string, unknown>) => void;
    saveDashboard: () => Promise<void>;
    createDashboard: (req: CreateDashboardRequest) => Promise<number>;
    deleteDashboard: (id: number) => Promise<void>;
    refreshDashboards: () => Promise<Dashboard[]>;
}

export const DashboardContext = createContext<DashboardContextValue | null>(null);

export function useDashboard() {
    const ctx = useContext(DashboardContext);
    if (!ctx) throw new Error('useDashboard must be used within DashboardProvider');
    return ctx;
}
