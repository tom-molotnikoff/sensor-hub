import {SidebarContext} from "./SidebarContextType.tsx";
import {useState} from "react";
import {useRoomForExpandedNav} from "../ui/tiers";

const collapsedKey = "sensor-hub.nav.collapsed";

function savedCollapsed(): boolean | null {
  const saved = localStorage.getItem(collapsedKey);
  return saved === "true" ? true : saved === "false" ? false : null;
}

type SidebarContextProviderProps = {
  children: React.ReactNode;
};


export function SidebarContextProvider({children}: SidebarContextProviderProps) {
  const [sidebarOpen, setSidebarOpen] = useState(false);
  const [saved, setSaved] = useState(savedCollapsed);
  const roomForExpandedNav = useRoomForExpandedNav();
  const collapsed = saved ?? !roomForExpandedNav;

  const toggleCollapsed = () => {
    localStorage.setItem(collapsedKey, String(!collapsed));
    setSaved(!collapsed);
  };

  return (
    <SidebarContext.Provider value={{open: sidebarOpen, setOpen: setSidebarOpen, collapsed, toggleCollapsed}}>
      {children}
    </SidebarContext.Provider>
  );
}
