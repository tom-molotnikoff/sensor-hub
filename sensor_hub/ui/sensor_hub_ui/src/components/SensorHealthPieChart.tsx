import {Cell, Legend, Pie, PieChart, LabelList} from 'recharts';
import type {Sensor} from "../gen/aliases";
import { useChartColours } from "../ui/theme/chartColours";

interface SensorHealthPieChartProps {
  sensors: Sensor[]
}

function SensorHealthPieChart({sensors}: SensorHealthPieChartProps) {

  const chartColours = useChartColours();
  const COLORS = chartColours.health;

  const data = [
    { name: 'Good', value: 0 },
    { name: 'Bad', value: 0 },
    { name: 'Unknown', value: 0 },
  ];

  for (const sensor of sensors) {
    if (sensor.health_status === 'good') {
      data[0].value += 1;
    } else if (sensor.health_status === 'bad') {
      data[1].value += 1;
    } else {
      data[2].value += 1;
    }
  }

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
        <Legend verticalAlign="top" height={36}/>
        <LabelList dataKey="value" position="outside" />
        {data.map((entry, index) => (
          <Cell key={`cell-${entry.name}`} fill={COLORS[index % COLORS.length]} />
        ))}
      </Pie>
    </PieChart>
  );
}

export default SensorHealthPieChart;