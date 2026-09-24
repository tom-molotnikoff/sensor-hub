import Typography from "@mui/material/Typography";
import WaterDropOutlined from "@mui/icons-material/WaterDropOutlined";
import AirOutlined from "@mui/icons-material/AirOutlined";
import { getWeatherInfo, inlineIcon } from "../tools/weatherIcons.ts";
import type { HourlyForecast } from "../hooks/useWeatherApi.ts";
import Strip, { StripCell, StripDetail } from "../ui/Strip";

type HourlyForecastDetailProps = {
  hours: HourlyForecast[];
};

function formatHour(timeStr: string): string {
  const date = new Date(timeStr);
  return date.toLocaleTimeString(undefined, {
    hour: "2-digit",
    minute: "2-digit",
    hour12: false,
  });
}


export default function HourlyForecastDetail({ hours }: HourlyForecastDetailProps) {
  if (hours.length === 0) {
    return (
      <Typography variant="body2" sx={{ color: "text.secondary" }}>
        No hourly data available for today.
      </Typography>
    );
  }

  return (
    <Strip label="Hourly forecast" size="sm">
      {hours.map((h) => {
        const { icon: WeatherIcon } = getWeatherInfo(h.weatherCode);
        return (
          <StripCell key={h.time} surface="outlined">
            <Typography variant="caption" sx={{ fontWeight: "fontWeightBold" }}>
              {formatHour(h.time)}
            </Typography>
            <WeatherIcon fontSize="small" sx={{ color: "primary.main" }} />
            <Typography variant="body2" sx={{ fontWeight: "fontWeightBold" }}>
              {Math.round(h.temperature)}°
            </Typography>
            <StripDetail>
              <Typography variant="caption" sx={{ color: "text.secondary" }}>
                Feels {Math.round(h.apparentTemperature)}°
              </Typography>
            </StripDetail>
            <Typography variant="caption">
              <WaterDropOutlined fontSize="inherit" sx={{ ...inlineIcon, color: "info.main" }} /> {h.precipitationProbability}%
            </Typography>
            <StripDetail>
              <Typography variant="caption">
                <AirOutlined fontSize="inherit" sx={{ ...inlineIcon, color: "text.secondary" }} /> {Math.round(h.windSpeed)}
              </Typography>
            </StripDetail>
          </StripCell>
        );
      })}
    </Strip>
  );
}
