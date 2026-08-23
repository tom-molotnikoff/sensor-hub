import { useCallback, useEffect, useState } from 'react';
import { apiClient } from '../gen/client';
import type { Dashboard, DashboardConfig, DashboardWidget, CreateDashboardRequest, UpdateDashboardRequest } from '../gen/aliases';
import { DEFAULT_BREAKPOINTS } from './constants';
import { logger } from '../tools/logger';
import { DashboardContext } from './DashboardContext';

const EMPTY_CONFIG: DashboardConfig = { widgets: [], breakpoints: DEFAULT_BREAKPOINTS };
const STORAGE_KEY = 'sensor-hub-active-dashboard-id';

function parseConfig(raw: string): DashboardConfig {
    try {
        return JSON.parse(raw) as DashboardConfig;
    } catch {
        logger.error('[Dashboard] Failed to parse config', raw);
        return EMPTY_CONFIG;
    }
}

function persistDashboardId(id: number | null) {
    if (id != null) {
        localStorage.setItem(STORAGE_KEY, String(id));
    } else {
        localStorage.removeItem(STORAGE_KEY);
    }
}

function getPersistedDashboardId(): number | null {
    const stored = localStorage.getItem(STORAGE_KEY);
    return stored ? Number(stored) : null;
}

export function DashboardProvider({ children }: { children: React.ReactNode }) {
    const [dashboards, setDashboards] = useState<Dashboard[]>([]);
    const [activeDashboard, setActiveDashboardState] = useState<Dashboard | null>(null);
    const [config, setConfig] = useState<DashboardConfig>(EMPTY_CONFIG);
    const [isEditing, setIsEditing] = useState(false);
    const [loading, setLoading] = useState(true);

    const refreshDashboards = useCallback(async () => {
        try {
            const { data: list } = await apiClient.GET('/dashboards');
            const dbs = (list as Dashboard[] | null) ?? [];
            setDashboards(dbs);
            return dbs;
        } catch (err) {
            logger.error('[Dashboard] Failed to load dashboards', err);
            return [];
        }
    }, []);

    useEffect(() => {
        void Promise.resolve().then(() => refreshDashboards()).then((list) => {
            if (list.length > 0) {
                const persistedId = getPersistedDashboardId();
                const persisted = persistedId != null ? list.find((d) => d.id === persistedId) : null;
                const defaultDb = persisted ?? list.find((d) => d.is_default) ?? list[0];
                setActiveDashboardState(defaultDb);
                setConfig(parseConfig(defaultDb.config));
                persistDashboardId(defaultDb.id);
            }
            setLoading(false);
        });
    }, [refreshDashboards]);

    const setActiveDashboard = useCallback((dashboard: Dashboard) => {
        setActiveDashboardState(dashboard);
        setConfig(parseConfig(dashboard.config));
        setIsEditing(false);
        persistDashboardId(dashboard.id);
    }, []);

    const updateWidgets = useCallback((widgets: DashboardWidget[]) => {
        setConfig((prev) => ({ ...prev, widgets }));
    }, []);

    const addWidget = useCallback((widget: DashboardWidget) => {
        setConfig((prev) => ({ ...prev, widgets: [...prev.widgets, widget] }));
    }, []);

    const removeWidget = useCallback((id: string) => {
        setConfig((prev) => ({ ...prev, widgets: prev.widgets.filter((w) => w.id !== id) }));
    }, []);

    const updateWidgetConfig = useCallback((id: string, widgetConfig: Record<string, unknown>) => {
        setConfig((prev) => ({
            ...prev,
            widgets: prev.widgets.map((w) => (w.id === id ? { ...w, config: widgetConfig } : w)),
        }));
    }, []);

    const saveDashboard = useCallback(async () => {
        if (!activeDashboard) return;
        const updateReq: UpdateDashboardRequest = { name: activeDashboard.name, config };
        await apiClient.PUT('/dashboards/{id}', { params: { path: { id: activeDashboard.id } }, body: updateReq as never });
        await refreshDashboards();
        setIsEditing(false);
    }, [activeDashboard, config, refreshDashboards]);

    const createDashboard = useCallback(async (req: CreateDashboardRequest) => {
        const { data: result } = await apiClient.POST('/dashboards', { body: req as never });
        const list = await refreshDashboards();
        const id = (result as { id: number }).id;
        const created = list.find((d) => d.id === id);
        if (created) {
            setActiveDashboardState(created);
            setConfig(parseConfig(created.config));
            persistDashboardId(created.id);
        }
        return id;
    }, [refreshDashboards]);

    const deleteDashboard = useCallback(async (id: number) => {
        await apiClient.DELETE('/dashboards/{id}', { params: { path: { id } } });
        const list = await refreshDashboards();
        if (activeDashboard?.id === id) {
            if (list.length > 0) {
                const next = list.find((d) => d.is_default) ?? list[0];
                setActiveDashboardState(next);
                setConfig(parseConfig(next.config));
                persistDashboardId(next.id);
            } else {
                setActiveDashboardState(null);
                setConfig(EMPTY_CONFIG);
                persistDashboardId(null);
            }
            setIsEditing(false);
        }
    }, [activeDashboard, refreshDashboards]);

    return (
        <DashboardContext.Provider value={{
            dashboards, activeDashboard, config, isEditing, loading,
            setIsEditing, setActiveDashboard, updateWidgets,
            addWidget, removeWidget, updateWidgetConfig,
            saveDashboard, createDashboard, deleteDashboard, refreshDashboards,
        }}>
            {children}
        </DashboardContext.Provider>
    );
}
