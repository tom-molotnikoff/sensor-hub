---
id: dashboards
title: Dashboards
sidebar_position: 9.5
---

# Dashboards

Dashboards let you build custom views of your Sensor Hub data by arranging
widgets on a drag-and-drop grid. Each dashboard is saved per-user and persists
across sessions.

## Widget configuration

Some widgets require or accept configuration:

- **sensorId** — which sensor to display (e.g. Health Timeline, Current
  Reading, Gauge, Uptime, Sensor Detail, Heatmap, Min/Max/Avg, Sensor Toggle)
- **sensorIds** — multiple sensors to display (e.g. Comparison Chart)
- **property** — which controllable binary property to switch (used by Sensor
  Toggle, defaults to `state` and is chosen from the selected sensor's
  available binary capabilities)
- **measurementType** — which measurement type to chart or display
  (e.g. `temperature`, `humidity`, `contact`, `power`)
- **timeRange** — time window for historical data: `1h`, `6h`, `24h`,
  `3d`, `7d`, `30d`, or `custom` with `customStart`/`customEnd` ISO dates
- **min / max** — scale range for the Gauge widget
- **scaleMin / scaleMax** — colour scale for the Heatmap widget
- **content** — markdown text for the Markdown Note widget

These are set in the widget settings dialog (gear icon) while in edit mode.

## Responsive layout

Every dashboard has one layout, a grid 12 columns wide. Each widget is stored
with its column (`x`), row (`y`), width in columns (`w`) and height in rows
(`h`). Any screen 900px or wider shows exactly that layout: a widget 3
columns wide takes a quarter of the grid at every desktop width, and resizing
the window never changes or saves the layout. You arrange widgets by dragging
and resizing them in edit mode, and saving stores the layout you see.

Narrower screens, such as a phone, show a projection of the same layout
instead of a second one. Every widget is full width, one per row, in reading
order: top to bottom by `y`, then left to right by `x`. Each widget type has a
fixed height on a phone, or takes the height of its content. There's nothing
to arrange on a phone, so edit mode there has no drag or resize handles.
