export interface StatusColour {
  strong: string;
  soft: string;
}

export type StatusKey = 'ok' | 'warn' | 'bad' | 'unknown' | 'info';

export type StatusPalette = Record<StatusKey, StatusColour>;

export interface ChartPalette {
  categorical: string[];
}

export interface NavPalette {
  bg: string;
  text: string;
  muted: string;
  hover: string;
  activeBg: string;
  activeText: string;
  indicator: string;
}

export type Scheme = 'light' | 'dark';

const amber: Record<Scheme, StatusColour> = {
  light: { strong: '#B26A00', soft: 'rgba(178,106,0,0.12)' },
  dark: { strong: '#F0B24A', soft: 'rgba(240,178,74,0.16)' },
};

export const statusPalettes: Record<Scheme, StatusPalette> = {
  light: {
    ok: { strong: '#2E7D32', soft: 'rgba(46,125,50,0.10)' },
    warn: amber.light,
    bad: { strong: '#B3261E', soft: 'rgba(179,38,30,0.10)' },
    unknown: amber.light,
    info: { strong: '#1B5E9E', soft: 'rgba(27,94,158,0.10)' },
  },
  dark: {
    ok: { strong: '#7BC67E', soft: 'rgba(123,198,126,0.14)' },
    warn: amber.dark,
    bad: { strong: '#F28B82', soft: 'rgba(242,139,130,0.14)' },
    unknown: amber.dark,
    info: { strong: '#7FB3E8', soft: 'rgba(127,179,232,0.14)' },
  },
};

export const chartPalettes: Record<Scheme, ChartPalette> = {
  light: {
    categorical: ['#D4451A', '#0288D1', '#388E3C', '#E65100', '#7B1FA2', '#00838F', '#5D4037', '#455A64'],
  },
  dark: {
    categorical: ['#ED5125', '#4FC3F7', '#81C784', '#FFB74D', '#CE93D8', '#4DD0E1', '#A1887F', '#90A4AE'],
  },
};

const charcoalNav: Omit<NavPalette, 'bg'> = {
  text: '#D9D3CC',
  muted: '#8F867D',
  hover: 'rgba(255,255,255,0.06)',
  activeBg: 'rgba(237,81,37,0.18)',
  activeText: '#FFFFFF',
  indicator: '#ED5125',
};

export const navPalettes: Record<Scheme, NavPalette> = {
  light: { bg: '#211E1B', ...charcoalNav },
  dark: { bg: '#121212', ...charcoalNav },
};
