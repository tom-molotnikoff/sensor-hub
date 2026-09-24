import { test as base } from '@playwright/test';

const days = 7;

function forecast() {
  const today = new Date();
  const dates = Array.from({ length: days }, (_, i) => new Date(today.getTime() + i * 86_400_000).toISOString().slice(0, 10));
  const hours = dates.slice(0, 2).flatMap((date) => Array.from({ length: 24 }, (_, h) => `${date}T${String(h).padStart(2, '0')}:00`));
  return {
    daily: {
      time: dates,
      weather_code: dates.map((_, i) => [0, 2, 3, 61, 80, 1, 45][i % 7]),
      temperature_2m_max: dates.map((_, i) => 14 + i),
      temperature_2m_min: dates.map((_, i) => 6 + i),
      precipitation_probability_max: dates.map((_, i) => (i * 13) % 100),
      wind_speed_10m_max: dates.map((_, i) => 10 + i * 2),
    },
    hourly: {
      time: hours,
      temperature_2m: hours.map((_, i) => 8 + (i % 24) / 3),
      weather_code: hours.map(() => 2),
      apparent_temperature: hours.map((_, i) => 7 + (i % 24) / 3),
      precipitation_probability: hours.map((_, i) => i % 30),
      wind_speed_10m: hours.map((_, i) => 8 + (i % 12)),
    },
  };
}

export const test = base.extend({
  page: async ({ page }, provide) => {
    await page.route('https://api.open-meteo.com/**', (route) => route.fulfill({ json: forecast() }));
    await provide(page);
  },
});

export { expect } from '@playwright/test';
export type { Locator, Page } from '@playwright/test';
