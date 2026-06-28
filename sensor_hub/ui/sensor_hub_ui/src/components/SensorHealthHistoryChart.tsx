import useSensorHealthHistory from "../hooks/useSensorHealthHistory.ts";
import type {Sensor} from "../gen/aliases";
import {type CSSProperties, useMemo} from "react";
import {
  CartesianGrid,
  Legend,
  Line,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
  Area,
  AreaChart,
  ReferenceArea,
} from "recharts";
import { useIsMobile } from "../hooks/useMobile";
import { useChartColours } from "../theme/chartColours";
import { buildHealthWindowModel, formatDurationShort, formatWindowLabel } from "../health/healthWindow";
import { useProperties } from "../hooks/useProperties.ts";
import { SignalTraceLoader } from "../dashboard/widget-loaders";

// Custom dot that only renders at transition points for lines with valid values
// eslint-disable-next-line @typescript-eslint/no-explicit-any
function TransitionDot(props: any) {
  const { cx, cy, payload, stroke, value } = props;
  // Only render if this is a transition AND this line has a value (not null)
  if (!payload?.isTransition || value === null) return null;
  return <circle cx={cx} cy={cy} r={4} fill={stroke} stroke={stroke} />;
}

interface SensorHealthHistoryChartProps {
  sensor: Sensor,
}

function SensorHealthHistoryChart({sensor}: SensorHealthHistoryChartProps) {
  const chartColours = useChartColours();
  const isMobile = useIsMobile();
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

  const lastChangeLabel = useMemo(() => {
    if (!model?.lastTransitionAt) return null;
    const elapsedMs = Math.max(0, Date.now() - new Date(model.lastTransitionAt).getTime());
    return `${formatDurationShort(elapsedMs)} ago`;
  }, [model]);

  return (
    <div data-testid="sensor-health-history-chart" style={graphContainerStyle}>
      {model && (
        <div style={summaryStyle}>
          <span>Window {formatWindowLabel(model.windowDurationMs)}</span>
          <span>Current {model.currentStatus}</span>
          {lastChangeLabel && <span>Last change {lastChangeLabel}</span>}
          <span>
            Good {formatDurationShort(model.durationsMs.good)} · Bad {formatDurationShort(model.durationsMs.bad)} · Unknown {formatDurationShort(model.durationsMs.unknown)}
          </span>
        </div>
      )}
      {historyLoading && mappedData.length === 0 ? (
        <div style={chartAreaStyle}>
          <SignalTraceLoader />
        </div>
      ) : !Array.isArray(mappedData) || mappedData.length === 0 ? (
        <></>
      ) : (
        <div style={chartAreaStyle}>
          <ResponsiveContainer width="100%" height="100%">
            <AreaChart data={mappedData} >
              <CartesianGrid stroke={chartColours.grid} strokeDasharray="3 3" />
              <ReferenceArea y1={-0.5} y2={0.5} fill={chartColours.health[2]} fillOpacity={0.15} />
              <ReferenceArea y1={0.5} y2={1.5} fill={chartColours.health[1]} fillOpacity={0.15} />
              <ReferenceArea y1={1.5} y2={2.5} fill={chartColours.health[0]} fillOpacity={0.15} />
              <XAxis
                dataKey="recorded_at"
                tickFormatter={(t) => {
                  if (!t) return "";
                  const date = new Date(t);
                  return isMobile 
                    ? date.toLocaleTimeString([], { hour: '2-digit' })
                    : date.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' });
                }}
                interval="preserveStartEnd"
                minTickGap={isMobile ? 30 : 50}
                tick={{ fontSize: isMobile ? 10 : 12 }}
                angle={isMobile ? -45 : 0}
                textAnchor={isMobile ? 'end' : 'middle'}
                height={isMobile ? 60 : 30}
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
                // eslint-disable-next-line @typescript-eslint/no-explicit-any
                formatter={(value: any, name: any) => {
                  if (name === 'healthValue') return [valueToLabel(Number(value)), 'Health'];
                  return [value, name];
                }}
                labelFormatter={(label) => {
                  if (!label) return '';
                  return new Date(label).toLocaleString();
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

              {/* Colored step lines per-state — only present where that state is active */}
              <Line type="step" dataKey="goodVal" stroke={chartColours.health[0]} dot={TransitionDot} strokeWidth={4} isAnimationActive={false} name="Good" />
              <Line type="step" dataKey="badVal" stroke={chartColours.health[1]} dot={TransitionDot} strokeWidth={4} isAnimationActive={false} name="Bad" />
              <Line type="step" dataKey="unknownVal" stroke={chartColours.health[2]} dot={TransitionDot} strokeWidth={4} isAnimationActive={false} name="Unknown" />

              <Legend />
            </AreaChart>
          </ResponsiveContainer>
        </div>
      )}
    </div>
  );
}

const graphContainerStyle: CSSProperties = {
  width: "100%",
  flex: 1,
  minHeight: 0,
  display: "flex",
  flexDirection: "column",
  gap: 8,
};

const summaryStyle: CSSProperties = {
  display: "flex",
  flexWrap: "wrap",
  gap: 12,
  fontSize: 12,
  color: "var(--mui-palette-text-secondary, rgba(0, 0, 0, 0.6))",
};

const chartAreaStyle: CSSProperties = {
  width: "100%",
  flex: 1,
  minHeight: 0,
  position: "relative",
};


export default SensorHealthHistoryChart;
