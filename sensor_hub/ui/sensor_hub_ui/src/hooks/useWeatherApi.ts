import { useCallback } from "react";
import { useAuth } from "../providers/AuthContext.tsx";
import { useScheduledQuery } from "./useScheduledQuery";

export type DailyForecast = {
  date: string;
  weatherCode: number;
  tempMax: number;
  tempMin: number;
  precipitationProbability: number;
  windSpeedMax: number;
};

export type HourlyForecast = {
  time: string;
  temperature: number;
  apparentTemperature: number;
  precipitationProbability: number;
  weatherCode: number;
  windSpeed: number;
};

export type WeatherForecastData = {
  daily: DailyForecast[];
  hourly: HourlyForecast[];
};

type UseWeatherApiResult = {
  data: WeatherForecastData | null;
  loading: boolean;
  error: string | null;
};

/** The forecast shares the dashboard's read budget, so it must not hold a slot indefinitely. */
const WEATHER_TIMEOUT_MS = 10000;

export function useWeatherApi(
  latitude: number,
  longitude: number
): UseWeatherApiResult {
  const { user } = useAuth();

  const fetcher = useCallback(async (signal: AbortSignal): Promise<WeatherForecastData> => {
    const params = new URLSearchParams({
      latitude: String(latitude),
      longitude: String(longitude),
      daily:
        "weather_code,temperature_2m_max,temperature_2m_min,precipitation_probability_max,wind_speed_10m_max",
      hourly:
        "temperature_2m,apparent_temperature,precipitation_probability,weather_code,wind_speed_10m",
      forecast_days: "7",
      timezone: "auto",
    });

    const url = `https://api.open-meteo.com/v1/forecast?${params.toString()}`;
    const resp = await fetch(url, {
      signal: AbortSignal.any([signal, AbortSignal.timeout(WEATHER_TIMEOUT_MS)]),
    });
    if (!resp.ok) throw new Error(`HTTP ${resp.status}`);
    const json = await resp.json();

    const dailyRaw = json.daily ?? {};
    const dailyTimes: string[] = dailyRaw.time ?? [];
    const daily: DailyForecast[] = dailyTimes.map((date: string, i: number) => ({
      date,
      weatherCode: dailyRaw.weather_code?.[i] ?? 0,
      tempMax: dailyRaw.temperature_2m_max?.[i] ?? 0,
      tempMin: dailyRaw.temperature_2m_min?.[i] ?? 0,
      precipitationProbability: dailyRaw.precipitation_probability_max?.[i] ?? 0,
      windSpeedMax: dailyRaw.wind_speed_10m_max?.[i] ?? 0,
    }));

    const hourlyRaw = json.hourly ?? {};
    const hourlyTimes: string[] = hourlyRaw.time ?? [];

    // The API's first daily date is in the requested local timezone; using UTC here would
    // pick the wrong day either side of midnight.
    const todayStr = dailyTimes[0] ?? new Date().toISOString().slice(0, 10);
    const hourly: HourlyForecast[] = hourlyTimes
      .map((time: string, i: number) => ({
        time,
        temperature: hourlyRaw.temperature_2m?.[i] ?? 0,
        apparentTemperature: hourlyRaw.apparent_temperature?.[i] ?? 0,
        precipitationProbability: hourlyRaw.precipitation_probability?.[i] ?? 0,
        weatherCode: hourlyRaw.weather_code?.[i] ?? 0,
        windSpeed: hourlyRaw.wind_speed_10m?.[i] ?? 0,
      }))
      .filter((h: HourlyForecast) => h.time.startsWith(todayStr));

    return { daily, hourly };
  }, [latitude, longitude]);

  const { data, error, isLoading } = useScheduledQuery(fetcher, {
    enabled: !!user && !!latitude && !!longitude,
    deps: [latitude, longitude],
  });

  return {
    data: data ?? null,
    loading: isLoading,
    error: error instanceof Error ? error.message : error != null ? String(error) : null,
  };
}

export default useWeatherApi;
