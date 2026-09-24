import type { SvgIconComponent } from "@mui/icons-material";
import WbSunnyOutlined from "@mui/icons-material/WbSunnyOutlined";
import CloudOutlined from "@mui/icons-material/CloudOutlined";
import Foggy from "@mui/icons-material/Foggy";
import GrainOutlined from "@mui/icons-material/GrainOutlined";
import WaterDropOutlined from "@mui/icons-material/WaterDropOutlined";
import AcUnitOutlined from "@mui/icons-material/AcUnitOutlined";
import ThunderstormOutlined from "@mui/icons-material/ThunderstormOutlined";

type WeatherInfo = {
  icon: SvgIconComponent;
  label: string;
};

const weatherCodeMap: Record<number, WeatherInfo> = {
  0: { icon: WbSunnyOutlined, label: "Clear sky" },
  1: { icon: WbSunnyOutlined, label: "Mainly clear" },
  2: { icon: CloudOutlined, label: "Partly cloudy" },
  3: { icon: CloudOutlined, label: "Overcast" },
  45: { icon: Foggy, label: "Fog" },
  48: { icon: Foggy, label: "Depositing rime fog" },
  51: { icon: GrainOutlined, label: "Light drizzle" },
  53: { icon: GrainOutlined, label: "Moderate drizzle" },
  55: { icon: GrainOutlined, label: "Dense drizzle" },
  56: { icon: GrainOutlined, label: "Light freezing drizzle" },
  57: { icon: GrainOutlined, label: "Dense freezing drizzle" },
  61: { icon: WaterDropOutlined, label: "Slight rain" },
  63: { icon: WaterDropOutlined, label: "Moderate rain" },
  65: { icon: WaterDropOutlined, label: "Heavy rain" },
  66: { icon: WaterDropOutlined, label: "Light freezing rain" },
  67: { icon: WaterDropOutlined, label: "Heavy freezing rain" },
  71: { icon: AcUnitOutlined, label: "Slight snow" },
  73: { icon: AcUnitOutlined, label: "Moderate snow" },
  75: { icon: AcUnitOutlined, label: "Heavy snow" },
  77: { icon: AcUnitOutlined, label: "Snow grains" },
  80: { icon: WaterDropOutlined, label: "Slight showers" },
  81: { icon: WaterDropOutlined, label: "Moderate showers" },
  82: { icon: WaterDropOutlined, label: "Violent showers" },
  85: { icon: AcUnitOutlined, label: "Slight snow showers" },
  86: { icon: AcUnitOutlined, label: "Heavy snow showers" },
  95: { icon: ThunderstormOutlined, label: "Thunderstorm" },
  96: { icon: ThunderstormOutlined, label: "Thunderstorm with slight hail" },
  99: { icon: ThunderstormOutlined, label: "Thunderstorm with heavy hail" },
};

const fallback: WeatherInfo = { icon: CloudOutlined, label: "Unknown" };

export function getWeatherInfo(code: number): WeatherInfo {
  return weatherCodeMap[code] ?? fallback;
}

export const inlineIcon = { verticalAlign: "middle" } as const;
