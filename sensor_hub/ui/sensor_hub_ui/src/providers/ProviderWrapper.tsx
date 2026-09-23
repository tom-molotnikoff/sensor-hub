import React from "react";
import {SensorContextProvider} from "./SensorContext.tsx";
import { ThemeProvider } from "@mui/material";
import { SidebarContextProvider } from "./SidebarContextProvider.tsx";
import LuxonLocalizationProvider from "./LuxonLocalizationProvider.tsx";
import { theme } from "../ui/theme";
import AuthProvider from './AuthProvider';
import NotificationProvider from './NotificationProvider';
import PropertiesProvider from './PropertiesProvider';
import CurrentReadingsProvider from './CurrentReadingsProvider';
import MeasurementTypesProvider from './MeasurementTypesProvider';

interface ProviderWrapperProps {
  children: React.ReactNode
}

function ProviderWrapper({ children }: ProviderWrapperProps) {
  return (
    <LuxonLocalizationProvider>
      <ThemeProvider theme={theme}>
        <AuthProvider>
          <NotificationProvider>
            <SidebarContextProvider>
              <SensorContextProvider>
                <PropertiesProvider>
                  <CurrentReadingsProvider>
                    <MeasurementTypesProvider>
                      {children}
                    </MeasurementTypesProvider>
                  </CurrentReadingsProvider>
                </PropertiesProvider>
              </SensorContextProvider>
            </SidebarContextProvider>
          </NotificationProvider>
        </AuthProvider>
      </ThemeProvider>
    </LuxonLocalizationProvider>
  )
}

export default ProviderWrapper