import Typography from "@mui/material/Typography";
import WaterDropOutlined from "@mui/icons-material/WaterDropOutlined";
import AirOutlined from "@mui/icons-material/AirOutlined";
import { getWeatherInfo } from "../tools/weatherIcons.ts";
import type { DailyForecast } from "../hooks/useWeatherApi.ts";
import { StripCell, StripDetail } from "../ui/Strip";

type DayForecastCardProps = {
  day: DailyForecast;
  isToday: boolean;
};

function formatDayName(dateStr: string): string {
  const date = new Date(dateStr + "T00:00:00");
  return date.toLocaleDateString(undefined, { weekday: "short" });
}

function formatShortDate(dateStr: string): string {
  const date = new Date(dateStr + "T00:00:00");
  return date.toLocaleDateString(undefined, { day: "numeric", month: "short" });
}

const inlineIcon = { verticalAlign: "middle" } as const;

export default function DayForecastCard({ day, isToday }: DayForecastCardProps) {
  const { icon: WeatherIcon, label } = getWeatherInfo(day.weatherCode);

  return (
    <StripCell highlighted={isToday}>
      <Typography variant="subtitle2" sx={{ fontWeight: "bold" }}>
        {isToday ? "Today" : formatDayName(day.date)}
      </Typography>
      <StripDetail>
        <Typography variant="caption" sx={{ color: "text.secondary" }}>
          {formatShortDate(day.date)}
        </Typography>
      </StripDetail>
      <WeatherIcon fontSize="large" sx={{ color: "primary.main" }} />
      <StripDetail>
        <Typography variant="caption" sx={{ color: "text.secondary" }}>
          {label}
        </Typography>
      </StripDetail>
      <Typography variant="body2" sx={{ fontWeight: "bold" }}>
        {Math.round(day.tempMax)}° / {Math.round(day.tempMin)}°
      </Typography>
      <Typography variant="caption">
        <WaterDropOutlined fontSize="inherit" sx={{ ...inlineIcon, color: "info.main" }} /> {day.precipitationProbability}%
      </Typography>
      <StripDetail>
        <Typography variant="caption">
          <AirOutlined fontSize="inherit" sx={{ ...inlineIcon, color: "text.secondary" }} /> {Math.round(day.windSpeedMax)} km/h
        </Typography>
      </StripDetail>
    </StripCell>
  );
}
