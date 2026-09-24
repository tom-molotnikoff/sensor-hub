import {
  LineChart,
  Line,
  XAxis,
  YAxis,
  Tooltip,
  CartesianGrid,
  Legend,
  type LegendPayload,
} from "recharts";
import React, {
  useEffect,
  useMemo,
  useReducer,
} from "react";
import { useReadingsData } from "../hooks/useReadingsData";
import { linesHiddenReducer } from "../reducers/LinesHiddenReducer";
import type {Sensor} from "../gen/aliases";
import type { DateTime } from "luxon";
import EmptyState from '../ui/EmptyState';
import ShowChartOutlinedIcon from "@mui/icons-material/ShowChartOutlined";
import { useChartColours } from "../ui/theme/chartColours";
import { theme } from "../ui/theme";
import ChartArea from "../ui/ChartArea";
import { WidgetSwap, SignalTraceLoader } from "../dashboard/widget-loaders";

const ReadingsChart = React.memo(function ReadingsChart({
  sensors,
  startDate,
  endDate,
  measurementType,
  aggregationFunction,
  pollIntervalMs,
  resolveTimeRange,
  onDataUpdate,
}: {
  sensors: Sensor[];
  startDate: DateTime | null;
  endDate: DateTime | null;
  measurementType?: string;
  aggregationFunction?: string;
  pollIntervalMs?: number;
  resolveTimeRange?: () => { startDate: DateTime | null; endDate: DateTime | null };
  onDataUpdate?: (date: Date) => void;
}) {

  const chartColours = useChartColours();

  const [linesHidden, setLinesHidden] = useReducer(linesHiddenReducer, {});

  const { mergedData: chartData, isLoading, error } = useReadingsData({
    startDate: startDate ? startDate : null,
    endDate: endDate ? endDate : null,
    sensors,
    measurementType,
    aggregationFunction,
    pollIntervalMs,
    resolveTimeRange,
    onDataUpdate,
  });

  const noData = !Array.isArray(chartData) || chartData.length === 0;
  const loading = sensors.length > 0 && isLoading && noData;

  const activeSensors = useMemo(() => {
    if (!Array.isArray(chartData) || chartData.length === 0) return [];
    return sensors.filter((s) =>
      chartData.some((entry) => entry[s.name] != null),
    );
  }, [chartData, sensors]);

  useEffect(() => {
    if (Object.keys(linesHidden).length !== 0) return;
    activeSensors.forEach((sensor) => {
      setLinesHidden({ type: "reset", key: sensor.name });
    });
  }, [activeSensors, linesHidden]);

  const legendClickHandler = (data: LegendPayload) => {
    setLinesHidden({ type: "toggle", key: data.dataKey as string });
  };

  const isBinaryData = useMemo(() => {
    if (!Array.isArray(chartData) || chartData.length === 0) return false;
    return activeSensors.every((s) =>
      chartData.every((entry) => {
        const v = entry[s.name];
        return v == null || v === 0 || v === 1;
      }),
    );
  }, [chartData, activeSensors]);

  const yAxisLabel = measurementType
    ? { value: measurementType.charAt(0).toUpperCase() + measurementType.slice(1), angle: -90, position: 'insideLeft' as const, style: { textAnchor: 'middle' as const, fontSize: theme.typography.caption.fontSize } }
    : undefined;

  const icon = <ShowChartOutlinedIcon fontSize="large" />;
  const emptyState = sensors.length === 0 ? (
    <EmptyState
      icon={icon}
      title="No sensors configured"
      description="Add a sensor to start seeing data here."
      actionLabel="Add a sensor"
      actionHref="/sensors-overview"
    />
  ) : error && noData ? (
    <EmptyState
      icon={icon}
      title="Couldn't load readings"
      description="Something went wrong fetching this chart. It will retry automatically."
    />
  ) : noData ? (
    <EmptyState
      icon={icon}
      title="No readings in selected date range"
      description="Try adjusting the date range or wait for new readings."
    />
  ) : activeSensors.length === 0 ? (
    <EmptyState
      icon={icon}
      title="No sensors have this reading type"
      description={measurementType
        ? `None of the available sensors report "${measurementType}" readings.`
        : "No matching sensor data found."}
    />
  ) : null;
  const showChart = !loading && emptyState === null;

  return (
    <ChartArea
      size="lg"
      placeholder={showChart ? undefined : (
        <WidgetSwap loading={loading} loader={<SignalTraceLoader />}>
          {emptyState}
        </WidgetSwap>
      )}
    >
      <LineChart data={chartData}>
        <CartesianGrid stroke={chartColours.grid} strokeDasharray="3 3" />
        <XAxis
          dataKey="time"
          tickFormatter={(t) => new Date(t).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })}
          interval="preserveStartEnd"
          minTickGap={50}
          tick={{ fontSize: theme.typography.caption.fontSize }}
        />
        <YAxis
          type="number"
          domain={isBinaryData ? [0, 1] : ['auto', 'auto']}
          ticks={isBinaryData ? [0, 1] : undefined}
          tickFormatter={isBinaryData ? (v: number) => (v === 1 ? 'true' : 'false') : undefined}
          tick={{ fontSize: theme.typography.caption.fontSize }}
          label={yAxisLabel}
        />
        <Tooltip />
        <Legend onClick={legendClickHandler} />
        {activeSensors.map((sensor, index) => (
          <Line
            key={sensor.name}
            type={isBinaryData ? 'stepAfter' : 'linear'}
            dataKey={sensor.name}
            stroke={chartColours.categorical[index % chartColours.categorical.length]}
            dot={false}
            connectNulls={true}
            animationEasing="ease-in-out"
            animationDuration={800}
            hide={linesHidden[sensor.name]}
            legendType="plainline"
          />
        ))}
      </LineChart>
    </ChartArea>
  );
});

export default ReadingsChart;
