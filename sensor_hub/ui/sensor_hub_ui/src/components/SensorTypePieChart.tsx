import { Cell, Legend, Pie, PieChart, LabelList } from "recharts";
import type { Sensor, DriverInfo } from "../gen/aliases";
import { useChartColours } from "../ui/theme/chartColours";

interface SensorTypePieChartProps {
  sensors: Sensor[];
  drivers?: DriverInfo[];
}

function SensorTypePieChart({ sensors, drivers }: SensorTypePieChartProps) {
  const chartColours = useChartColours();
  const COLORS = chartColours.categorical;

  const driverDisplayNames = new Map(
    (drivers ?? []).map((d) => [d.type, d.display_name]),
  );

  const counts = new Map<string, number>();
  for (const sensor of sensors) {
    const key = sensor.sensor_driver ?? "Unknown";
    counts.set(key, (counts.get(key) ?? 0) + 1);
  }

  const data = [...counts.entries()]
    .map(([driver, value]) => ({
      name: driverDisplayNames.get(driver) ?? driver,
      value,
    }))
    .sort((a, b) => b.value - a.value);

  return (
    <PieChart>
      <Pie
        data={data}
        innerRadius="40%"
        outerRadius="55%"
        paddingAngle={5}
        dataKey="value"
        nameKey="name"
        cx="50%"
        cy="50%"
      >
        <Legend verticalAlign="top" height={36} />
        <LabelList dataKey="value" position="outside" />
        {data.map((entry, index) => (
          <Cell
            key={`cell-${entry.name}`}
            fill={COLORS[index % COLORS.length]}
          />
        ))}
      </Pie>
    </PieChart>
  );
}

export default SensorTypePieChart;
