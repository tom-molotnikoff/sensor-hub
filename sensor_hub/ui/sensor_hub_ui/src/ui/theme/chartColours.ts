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

const lightColours: ChartColours = {
  categorical: ['#D4451A', '#0288D1', '#388E3C', '#E65100', '#7B1FA2', '#00838F', '#5D4037', '#455A64'],
  health: ['#2E7D32', '#C62828', '#E65100'],
  stat: ['#0288D1', '#5C5C5C', '#C62828'],
  grid: '#D9D0C7',
  axisText: '#5C5C5C',
  noData: '#E0D8D0',
};

const darkColours: ChartColours = {
  categorical: ['#ED5125', '#4FC3F7', '#81C784', '#FFB74D', '#CE93D8', '#4DD0E1', '#A1887F', '#90A4AE'],
  health: ['#66BB6A', '#EF5350', '#FFA726'],
  stat: ['#4FC3F7', '#A0A0A0', '#EF5350'],
  grid: '#333333',
  axisText: '#A0A0A0',
  noData: '#333333',
};

export function useChartColours(): ChartColours {
  const isDark = useIsDark();
  return isDark ? darkColours : lightColours;
}

export type { ChartColours };
