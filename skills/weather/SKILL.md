---
name: weather
description: "Get current weather and forecasts using free services (no API key required)."
metadata:
  version: "1.0.0"
  requires:
    bins: ["curl"]
---

# Weather Skill

Two free weather services, no API keys needed.

## wttr.in (primary)

Quick one-liner:

```bash
curl -s "wttr.in/London?format=3"
# Output: London: +8C
```

Compact format:

```bash
curl -s "wttr.in/London?format=%l:+%c+%t+%h+%w"
# Output: London: +8C 71% 5km/h
```

Full forecast:

```bash
curl -s "wttr.in/London?T"
```

### Format codes

`%c` condition, `%t` temp, `%h` humidity, `%w` wind, `%l` location, `%m` moon phase

### Tips

- URL-encode spaces: `wttr.in/New+York`
- Airport codes: `wttr.in/JFK`
- Units: `?m` (metric) `?u` (USCS/imperial)
- Today only: `?1` / Current only: `?0`
- PNG image: `curl -s "wttr.in/Berlin.png" -o /tmp/weather.png`

## Open-Meteo (fallback, JSON)

Free, no key, good for programmatic use:

```bash
curl -s "https://api.open-meteo.com/v1/forecast?latitude=51.5&longitude=-0.12&current_weather=true"
```

Find coordinates for a city, then query. Returns JSON with temp, windspeed, weathercode.

Docs: https://open-meteo.com/en/docs
