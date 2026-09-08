import React from "react";
import {SensorContextProvider} from "./SensorContext.tsx";
import { ThemeProvider } from "@mui/material";
import { SidebarContextProvider } from "./SidebarContextProvider.tsx";
import LuxonLocalizationProvider from "./LuxonLocalizationProvider.tsx";
import {createTheme} from "@mui/material/styles";
import AuthProvider from './AuthProvider';
import NotificationProvider from './NotificationProvider';
import PropertiesProvider from './PropertiesProvider';
import CurrentReadingsProvider from './CurrentReadingsProvider';
import MeasurementTypesProvider from './MeasurementTypesProvider';

interface ProviderWrapperProps {
  children: React.ReactNode
}

function ProviderWrapper({ children }: ProviderWrapperProps) {
  const theme = createTheme({
    cssVariables: {
      colorSchemeSelector: 'class',
    },
    components: {
      MuiAppBar: {
        styleOverrides: {
          root: {
            backgroundImage: 'none',
          },
        },
        defaultProps: {
          color: 'primary',
          enableColorOnDark: true,
        },
      },
    },
    colorSchemes: {
      light: {
        palette: {
          primary: {
            main: '#D4451A',
            light: '#ED5125',
            dark: '#B33612',
            contrastText: '#FFFFFF',
          },
          background: {
            default: '#F5F0EB',
            paper: '#FFFFFF',
          },
          text: {
            primary: '#1A1A1A',
            secondary: '#5C5C5C',
          },
          divider: '#D9D0C7',
          action: {
            hover: 'rgba(212,69,26,0.06)',
          },
        },
      },
      dark: {
        palette: {
          primary: {
            main: '#ED5125',
            light: '#F47A56',
            dark: '#C43D18',
            contrastText: '#FFFFFF',
          },
          background: {
            default: '#1A1A1A',
            paper: '#242424',
          },
          text: {
            primary: '#E8E8E8',
            secondary: '#A0A0A0',
          },
          divider: '#333333',
          action: {
            hover: 'rgba(237,81,37,0.08)',
          },
        },
      },
    },
  });

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