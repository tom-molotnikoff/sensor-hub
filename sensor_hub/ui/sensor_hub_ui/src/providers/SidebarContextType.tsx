import {createContext, type Dispatch, type SetStateAction} from "react";

type SidebarContextType = {
  open: boolean;
  setOpen: Dispatch<SetStateAction<boolean>>;
  collapsed: boolean;
  toggleCollapsed: () => void;
};

export const SidebarContext = createContext<SidebarContextType>({
  open: false,
  setOpen: () => {},
  collapsed: false,
  toggleCollapsed: () => {},
});
