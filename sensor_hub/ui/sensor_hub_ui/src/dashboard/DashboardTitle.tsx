import { useState } from 'react';
import { MenuItem } from '@mui/material';
import AnchoredMenu from '../ui/AnchoredMenu';
import PageTitleButton from '../ui/PageTitleButton';
import { useDashboard } from './DashboardContext';

const buttonId = 'dashboard-switcher-button';
const menuId = 'dashboard-switcher-menu';

export default function DashboardTitle() {
    const { dashboards, activeDashboard, setActiveDashboard } = useDashboard();
    const [anchorEl, setAnchorEl] = useState<HTMLElement | null>(null);

    if (!activeDashboard) return null;

    return (
        <>
            <PageTitleButton
                id={buttonId}
                label={activeDashboard.name}
                menuId={menuId}
                open={anchorEl !== null}
                onClick={(e) => setAnchorEl(e.currentTarget)}
            />
            <AnchoredMenu
                id={menuId}
                anchorEl={anchorEl}
                onClose={() => setAnchorEl(null)}
                width="sm"
                placement="below-start"
                labelledBy={buttonId}
                variant="selectedMenu"
            >
                {dashboards.map((d) => (
                    <MenuItem
                        key={d.id}
                        selected={d.id === activeDashboard.id}
                        onClick={() => {
                            setAnchorEl(null);
                            if (d.id !== activeDashboard.id) setActiveDashboard(d);
                        }}
                    >
                        {d.name}{d.is_default ? ' ★' : ''}
                    </MenuItem>
                ))}
            </AnchoredMenu>
        </>
    );
}
