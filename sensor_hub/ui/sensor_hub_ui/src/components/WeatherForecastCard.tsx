import { useState } from "react";
import Button from "@mui/material/Button";
import Alert from "@mui/material/Alert";
import ExpandMoreOutlined from "@mui/icons-material/ExpandMoreOutlined";
import ExpandLessOutlined from "@mui/icons-material/ExpandLessOutlined";
import { useProperties } from "../hooks/useProperties.ts";
import { useWeatherApi } from "../hooks/useWeatherApi.ts";
import DayForecastCard from "./DayForecastCard.tsx";
import HourlyForecastDetail from "./HourlyForecastDetail.tsx";
import Card from '../ui/Card';
import EmptyState from '../ui/EmptyState';
import Inline from '../ui/Inline';
import Stack from '../ui/Stack';
import Strip from '../ui/Strip';
import { WeatherColumnsLoader } from "../ui/loaders";
import { useWidgetStateReport } from "../dashboard/WidgetContext";

export default function WeatherForecastCard() {
  const properties = useProperties();
  const [showHourly, setShowHourly] = useState(true);

  const latStr = properties["weather.latitude"] ?? "";
  const lonStr = properties["weather.longitude"] ?? "";
  const locationName = properties["weather.location.name"] ?? "Weather";

  const lat = parseFloat(latStr);
  const lon = parseFloat(lonStr);
  const hasLocation = !isNaN(lat) && !isNaN(lon);
  useWidgetStateReport(hasLocation ? null : 'populated');

  const { data, loading, error } = useWeatherApi(
    hasLocation ? lat : 0,
    hasLocation ? lon : 0
  );

  const todayStr = new Date().toISOString().slice(0, 10);

  return (
    <Card title={`Weather - ${locationName}`}>
      {!hasLocation && (
        <EmptyState
          title="Location not configured"
          description="Set weather.latitude, weather.longitude, and weather.location.name in Settings → Application Properties."
          actionLabel="Go to Settings"
          actionHref="/settings"
          size="sm"
        />
      )}
      {hasLocation && loading && !data && <WeatherColumnsLoader />}
      {hasLocation && (error || data) && (
        <Stack>
          {error && (
            <Alert severity="warning">
              Could not load weather data: {error}
            </Alert>
          )}
          {data && (
            <Strip label="Daily forecast" size="md">
              {data.daily.map((day) => (
                <DayForecastCard
                  key={day.date}
                  day={day}
                  isToday={day.date === todayStr}
                />
              ))}
            </Strip>
          )}
          {data && data.hourly.length > 0 && (
            <Inline>
              <Button
                size="small"
                onClick={() => setShowHourly((prev) => !prev)}
                endIcon={showHourly ? <ExpandLessOutlined /> : <ExpandMoreOutlined />}
              >
                {showHourly ? "Hide hourly detail" : "Show today's hourly forecast"}
              </Button>
            </Inline>
          )}
          {data && data.hourly.length > 0 && showHourly && (
            <HourlyForecastDetail hours={data.hourly} />
          )}
        </Stack>
      )}
    </Card>
  );
}
