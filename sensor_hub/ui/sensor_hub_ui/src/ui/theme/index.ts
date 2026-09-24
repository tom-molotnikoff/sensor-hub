import { createTheme, type TypographyStyle } from '@mui/material/styles';
import { breakpointValues, compactMediaQuery, wideMediaQuery, type TierValues } from '../tiers';
import { density, type Density } from './tokens';

declare module '@mui/material/styles' {
  interface Theme {
    density: Density;
  }
  interface ThemeOptions {
    density?: Density;
  }
  interface TypographyVariants {
    pageTitle: TypographyStyle;
    cardTitle: TypographyStyle;
    sectionTitle: TypographyStyle;
    body: TypographyStyle;
    bodySmall: TypographyStyle;
    metricSm: TypographyStyle;
    metricMd: TypographyStyle;
    metricLg: TypographyStyle;
  }
  interface TypographyVariantsOptions {
    pageTitle?: TypographyStyle;
    cardTitle?: TypographyStyle;
    sectionTitle?: TypographyStyle;
    body?: TypographyStyle;
    bodySmall?: TypographyStyle;
    metricSm?: TypographyStyle;
    metricMd?: TypographyStyle;
    metricLg?: TypographyStyle;
  }
}

declare module '@mui/material/Typography' {
  interface TypographyPropsVariantOverrides {
    pageTitle: true;
    cardTitle: true;
    sectionTitle: true;
    body: true;
    bodySmall: true;
    metricSm: true;
    metricMd: true;
    metricLg: true;
  }
}

function typeScale(fontSize: number | TierValues<number>, fontWeight: number): TypographyStyle {
  if (typeof fontSize === 'number') return { fontSize: `${fontSize}px`, fontWeight };
  return { fontSize: `${fontSize.compact}px`, fontWeight, [wideMediaQuery]: { fontSize: `${fontSize.wide}px` } };
}

function metric(fontSize: number): TypographyStyle {
  return { ...typeScale(fontSize, 700), fontVariantNumeric: 'tabular-nums' };
}

export const theme = createTheme({
  cssVariables: {
    colorSchemeSelector: 'class',
  },
  breakpoints: {
    values: breakpointValues,
  },
  density,
  typography: {
    pageTitle: typeScale({ compact: 18, wide: 20 }, 500),
    cardTitle: typeScale({ compact: 18, wide: 20 }, 600),
    sectionTitle: typeScale(15, 600),
    body: typeScale(15, 400),
    bodySmall: typeScale(13, 400),
    caption: typeScale(12, 400),
    metricSm: metric(24),
    metricMd: metric(36),
    metricLg: metric(56),
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
    MuiDialog: {
      styleOverrides: {
        paper: {
          [compactMediaQuery]: {
            margin: 0,
            width: '100%',
            maxWidth: '100%',
            height: '100%',
            maxHeight: 'none',
            borderRadius: 0,
          },
        },
      },
    },
    MuiTypography: {
      defaultProps: {
        variantMapping: {
          pageTitle: 'h1',
          cardTitle: 'h2',
          sectionTitle: 'h3',
          body: 'p',
          bodySmall: 'p',
          metricSm: 'span',
          metricMd: 'span',
          metricLg: 'span',
        },
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
