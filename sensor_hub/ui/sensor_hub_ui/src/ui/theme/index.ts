import { createTheme, type TypographyStyle } from '@mui/material/styles';
import type {} from '@mui/x-data-grid/themeAugmentation';
import { breakpointValues, compactMediaQuery, wideMediaQuery, type TierValues } from '../tiers';
import { chartPalettes, navPalettes, statusPalettes, type ChartPalette, type NavPalette, type StatusPalette } from './palette';
import { density, type Density } from './tokens';

export type { StatusKey } from './palette';

declare module '@mui/material/styles' {
  interface Palette {
    status: StatusPalette;
    chart: ChartPalette;
    nav: NavPalette;
  }
  interface PaletteOptions {
    status?: StatusPalette;
    chart?: ChartPalette;
    nav?: NavPalette;
  }
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

const byTier = (value: number | TierValues<number>): TierValues<number> =>
  typeof value === 'number' ? { compact: value, wide: value } : value;

function typeScale(fontSize: number | TierValues<number>, fontWeight: number | TierValues<number>): TypographyStyle {
  const size = byTier(fontSize);
  const weight = byTier(fontWeight);
  const compact = { fontSize: `${size.compact}px`, fontWeight: weight.compact };
  if (size.compact === size.wide && weight.compact === weight.wide) return compact;
  return { ...compact, [wideMediaQuery]: { fontSize: `${size.wide}px`, fontWeight: weight.wide } };
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
    pageTitle: typeScale({ compact: 18, wide: 24 }, { compact: 500, wide: 600 }),
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
    MuiDataGrid: {
      defaultProps: {
        pageSizeOptions: [5, 10, 25, 50, 100],
        initialState: { pagination: { paginationModel: { pageSize: 10, page: 0 } } },
        getRowHeight: () => 'auto',
      },
      styleOverrides: {
        cell: {
          paddingTop: 6,
          paddingBottom: 6,
        },
      },
    },
    MuiDialogContent: {
      styleOverrides: {
        root: {
          '.MuiDialogTitle-root + &': {
            paddingTop: 8,
          },
        },
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
        status: statusPalettes.light,
        chart: chartPalettes.light,
        nav: navPalettes.light,
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
        status: statusPalettes.dark,
        chart: chartPalettes.dark,
        nav: navPalettes.dark,
      },
    },
  },
});
