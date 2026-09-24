import { chartPalettes, statusPalettes, type Scheme } from './palette';
import { useIsDark } from './useIsDark';

interface ChartColours {
  /** 8-colour cycle for categorical data (line charts, pie charts) */
  categorical: string[];
  /** Semantic health colours: [good, bad, unknown] */
  health: [string, string, string];
  /** Stat colours: [min/cold, neutral, max/hot] */
  stat: [string, string, string];
  /** CartesianGrid / axis stroke */
  grid: string;
  /** Axis tick and label colour */
  axisText: string;
  /** Disabled / no-data background */
  noData: string;
}

function fromPalette(scheme: Scheme, rest: Omit<ChartColours, 'categorical' | 'health'>): ChartColours {
  const status = statusPalettes[scheme];
  const chart = chartPalettes[scheme];
  return {
    categorical: chart.categorical,
    health: [status.ok.strong, status.bad.strong, status.unknown.strong],
    ...rest,
  };
}

const lightColours = fromPalette('light', {
  stat: ['#0288D1', '#5C5C5C', '#C62828'],
  grid: '#D9D0C7',
  axisText: '#5C5C5C',
  noData: '#E0D8D0',
});

const darkColours = fromPalette('dark', {
  stat: ['#4FC3F7', '#A0A0A0', '#EF5350'],
  grid: '#333333',
  axisText: '#A0A0A0',
  noData: '#333333',
});

const heatStops = [
  [33, 102, 172],
  [44, 162, 195],
  [68, 179, 96],
  [253, 200, 47],
  [215, 48, 39],
];

export function heatColour(ratio: number): string {
  const clamped = Math.max(0, Math.min(1, ratio));
  const idx = clamped * (heatStops.length - 1);
  const lo = Math.min(Math.floor(idx), heatStops.length - 2);
  const t = idx - lo;
  const [r, g, b] = heatStops[lo].map((channel, i) => Math.round(channel + t * (heatStops[lo + 1][i] - channel)));
  return `rgb(${r},${g},${b})`;
}

export function useChartColours(): ChartColours {
  const isDark = useIsDark();
  return isDark ? darkColours : lightColours;
}

export type { ChartColours };
