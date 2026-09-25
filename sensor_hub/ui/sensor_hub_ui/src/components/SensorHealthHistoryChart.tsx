import useSensorHealthHistory from "../hooks/useSensorHealthHistory.ts";
import type {Sensor} from "../gen/aliases";
import { useEffect, useMemo, useState } from "react";
import { Typography } from "@mui/material";
import {
  CartesianGrid,
  Legend,
  Line,
  Tooltip,
  XAxis,
  YAxis,
  Area,
  AreaChart,
  ReferenceArea,
} from "recharts";
import { useChartColours } from "../ui/theme/chartColours";
import { theme } from "../ui/theme";
import ChartArea from "../ui/ChartArea";
import ChartTooltip from "../ui/ChartTooltip";
import Inline from "../ui/Inline";
import Stack from "../ui/Stack";
import EmptyState from "../ui/EmptyState";
import MonitorHeartOutlinedIcon from "@mui/icons-material/MonitorHeartOutlined";
import { buildHealthWindowModel, formatDurationShort, formatWindowLabel } from "../health/healthWindow";
import { useProperties } from "../hooks/useProperties.ts";
import { SignalTraceLoader } from "../ui/loaders";

// eslint-disable-next-line @typescript-eslint/no-explicit-any
function TransitionDot(props: any) {
  const { cx, cy, payload, stroke, value } = props;
  if (!payload?.isTransition || value === null) return null;
  return <circle cx={cx} cy={cy} r={4} fill={stroke} stroke={stroke} />;
}

interface SensorHealthHistoryChartProps {
  sensor: Sensor,
}

function SensorHealthHistoryChart({sensor}: SensorHealthHistoryChartProps) {
  const chartColours = useChartColours();
  const properties = useProperties();

  const [healthHistoryData, , historyLoading] = useSensorHealthHistory(sensor.name);

  const model = useMemo(() => {
    if (!Array.isArray(healthHistoryData) || healthHistoryData.length === 0) return null;
    const now = new Date();
    const sortedByRecordedAt = [...healthHistoryData].sort((a, b) => {
      const dateA = new Date(a.recorded_at).getTime();
      const dateB = new Date(b.recorded_at).getTime();
      return dateA - dateB;
    });
    const configuredRetentionDays = Number.parseInt(properties['health.history.retention.days'] ?? '', 10);
    const windowStart = Number.isFinite(configuredRetentionDays) && configuredRetentionDays > 0
      ? new Date(now.getTime() - configuredRetentionDays * 24 * 60 * 60 * 1000)
      : new Date(sortedByRecordedAt[0].recorded_at);

    return buildHealthWindowModel(sortedByRecordedAt, {
      windowStart,
      now,
    });
  }, [healthHistoryData, properties]);

  const mappedData = useMemo(() => {
    if (!model) return [];

    const mapStatusToValue = (s: string | undefined | null) => {
      if (!s) return 0;
      const lower = s.toString().toLowerCase();
      if (lower === "good") return 2;
      if (lower === "bad") return 1;
      if (lower === "unknown") return 0;
      return 0;
    };

    return model.points.map((h, index) => {
      const recorded = h.recorded_at;
      const status = h.health_status;
      const value = mapStatusToValue(status);
      const prevValue = index > 0 ? mapStatusToValue(model.points[index - 1].health_status) : null;
      const isTransition = prevValue === null || prevValue !== value;
      return {
        ...h,
        recorded_at: recorded,
        health_status: status,
        healthValue: value,
        isTransition,
        goodVal: value === 2 ? 2 : null,
        badVal: value === 1 ? 1 : null,
        unknownVal: value === 0 ? 0 : null,
      };
    });
  }, [model]);

  const valueToLabel = (v: number) => {
    if (v === 2) return "good";
    if (v === 1) return "bad";
    return "unknown";
  };

  // The label depends on the wall clock, which render must not read, so it is
  // computed once per model change in this effect. The synchronous setState is
  // deliberate, not an oversight.
  const [lastChangeLabel, setLastChangeLabel] = useState<string | null>(null);
  /* eslint-disable react-hooks/set-state-in-effect */
  useEffect(() => {
    if (!model?.lastTransitionAt) {
      setLastChangeLabel(null);
      return;
    }
    const elapsedMs = Math.max(0, Date.now() - new Date(model.lastTransitionAt).getTime());
    setLastChangeLabel(`${formatDurationShort(elapsedMs)} ago`);
  }, [model]);
  /* eslint-enable react-hooks/set-state-in-effect */

  const loadingFirst = historyLoading && mappedData.length === 0;

  return (
    <Stack>
      {model && (
        <Inline>
          {[
            `Window ${formatWindowLabel(model.windowDurationMs)}`,
            `Current ${model.currentStatus}`,
            ...(lastChangeLabel ? [`Last change ${lastChangeLabel}`] : []),
            `Good ${formatDurationShort(model.durationsMs.good)} · Bad ${formatDurationShort(model.durationsMs.bad)} · Unknown ${formatDurationShort(model.durationsMs.unknown)}`,
          ].map((text) => (
            <Typography key={text} variant="caption" sx={{ color: "text.secondary" }}>{text}</Typography>
          ))}
        </Inline>
      )}
      {!loadingFirst && mappedData.length === 0 && (
        <EmptyState
          icon={<MonitorHeartOutlinedIcon fontSize="large" />}
          title="No health history yet"
          description="Health changes will appear here once the sensor reports."
        />
      )}
      {(loadingFirst || mappedData.length > 0) && (
        <ChartArea size="lg" placeholder={loadingFirst ? <SignalTraceLoader /> : undefined}>
          <AreaChart data={mappedData}>
            <CartesianGrid stroke={chartColours.grid} strokeDasharray="3 3" />
            <ReferenceArea y1={-0.5} y2={0.5} fill={chartColours.health[2]} fillOpacity={0.15} />
            <ReferenceArea y1={0.5} y2={1.5} fill={chartColours.health[1]} fillOpacity={0.15} />
            <ReferenceArea y1={1.5} y2={2.5} fill={chartColours.health[0]} fillOpacity={0.15} />
            <XAxis
              dataKey="recorded_at"
              tickFormatter={(t) => {
                if (!t) return "";
                return new Date(t).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' });
              }}
              interval="preserveStartEnd"
              minTickGap={50}
              tick={{ fontSize: theme.typography.caption.fontSize }}
            />
            <YAxis
              type="number"
              dataKey="healthValue"
              domain={[-0.5, 2.5]}
              ticks={[0, 1, 2]}
              tickFormatter={(v) => valueToLabel(Number(v))}
              allowDataOverflow={false}
              width={80}
            />
            <Tooltip
              content={ChartTooltip}
              formatter={(value, name) => {
                if (name === 'healthValue') return [valueToLabel(Number(value)), 'Health'];
                return [value, name];
              }}
            />
            <Area
              type="step"
              dataKey="healthValue"
              stroke="transparent"
              fill="transparent"
              strokeWidth={2}
              dot={false}
              isAnimationActive={false}
            />
            <Line type="step" dataKey="goodVal" stroke={chartColours.health[0]} dot={TransitionDot} strokeWidth={4} isAnimationActive={false} name="Good" />
            <Line type="step" dataKey="badVal" stroke={chartColours.health[1]} dot={TransitionDot} strokeWidth={4} isAnimationActive={false} name="Bad" />
            <Line type="step" dataKey="unknownVal" stroke={chartColours.health[2]} dot={TransitionDot} strokeWidth={4} isAnimationActive={false} name="Unknown" />
            <Legend />
          </AreaChart>
        </ChartArea>
      )}
    </Stack>
  );
}

export default SensorHealthHistoryChart;
