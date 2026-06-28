import { useState } from "react";
import Box from "@mui/material/Box";
import Button from "@mui/material/Button";
import Alert from "@mui/material/Alert";
import ExpandMoreOutlined from "@mui/icons-material/ExpandMoreOutlined";
import ExpandLessOutlined from "@mui/icons-material/ExpandLessOutlined";
import LayoutCard from "../tools/LayoutCard.tsx";
import { TypographyH2 } from "../tools/Typography.tsx";
import { useProperties } from "../hooks/useProperties.ts";
import { useWeatherApi } from "../hooks/useWeatherApi.ts";
import { useIsMobile } from "../hooks/useMobile.ts";
import DayForecastCard from "./DayForecastCard.tsx";
import HourlyForecastDetail from "./HourlyForecastDetail.tsx";
import EmptyState from "./EmptyState.tsx";
import { WeatherColumnsLoader } from "../dashboard/widget-loaders";

export default function WeatherForecastCard({ showTitle = true }: { showTitle?: boolean }) {
  const properties = useProperties();
  const [showHourly, setShowHourly] = useState(true);
  const isMobile = useIsMobile();

  const latStr = properties["weather.latitude"] ?? "";
  const lonStr = properties["weather.longitude"] ?? "";
  const locationName = properties["weather.location.name"] ?? "Weather";

  const lat = parseFloat(latStr);
  const lon = parseFloat(lonStr);
  const hasLocation = !isNaN(lat) && !isNaN(lon);

  const { data, loading, error } = useWeatherApi(
    hasLocation ? lat : 0,
    hasLocation ? lon : 0
  );

  const todayStr = new Date().toISOString().slice(0, 10);

  return (
    <LayoutCard variant="secondary" changes={{ height: '100%', width: '100%', overflow: 'hidden' }}>
      {showTitle && <TypographyH2 changes={{ textAlign: "center", flexShrink: 0 }}>Weather — {locationName}</TypographyH2>}
      {!hasLocation && (
        <EmptyState
          title="Location not configured"
          description="Set weather.latitude, weather.longitude, and weather.location.name in Settings → Application Properties."
          actionLabel="Go to Settings"
          actionHref="/settings"
          minHeight={120}
        />
      )}
      {hasLocation && loading && !data && (
        <Box sx={{ flex: 1, minHeight: 0 }}>
          <WeatherColumnsLoader />
        </Box>
      )}
      {hasLocation && error && (
        <Alert severity="warning" sx={{ my: 1 }}>
          Could not load weather data: {error}
        </Alert>
      )}
      {hasLocation && data && (
        <Box sx={{ flex: 1, minHeight: 0, overflow: 'auto', display: 'flex', flexDirection: 'column', gap: 1 }}>
          <Box
            sx={{
              display: "flex",
              gap: 1.5,
              overflowX: "auto",
              flexShrink: 0,
              pb: 1,
              "&::-webkit-scrollbar": { height: 6 },
              "&::-webkit-scrollbar-thumb": {
                borderRadius: 3,
                bgcolor: "action.disabled",
              },
            }}
          >
            {data.daily.map((day) => (
              <DayForecastCard
                key={day.date}
                day={day}
                isToday={day.date === todayStr}
                compact={isMobile}
              />
            ))}
          </Box>

          {data.hourly.length > 0 && (
            <Box sx={{ mt: 1 }}>
              <Button
                size="small"
                onClick={() => setShowHourly((prev) => !prev)}
                endIcon={
                  showHourly ? (
                    <ExpandLessOutlined />
                  ) : (
                    <ExpandMoreOutlined />
                  )
                }
              >
                {showHourly
                  ? "Hide hourly detail"
                  : "Show today's hourly forecast"}
              </Button>
              {showHourly && (
                <Box sx={{ mt: 1 }}>
                  <HourlyForecastDetail hours={data.hourly} compact={isMobile} />
                </Box>
              )}
            </Box>
          )}
        </Box>
      )}
    </LayoutCard>
  );
}
