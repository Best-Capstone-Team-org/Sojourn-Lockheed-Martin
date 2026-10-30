# Sojourn: Deep-Space Reverse Engineering Game Platform

## Team Members

- Macsen Casaus
- Susan Hamilton
- Isaac Lagoy
- Owen Shadburne
- David Zhang

## Sponsor / Mentor

**Sponsor:** Lockheed Martin

## Project Goal

Aerospace and defense organizations like Lockheed Martin need engineers who can analyze software when its source code is gone, yet hands-on training for reverse engineering is scarce. Sojourn is an educational game that casts players as mission-operations engineers saving an aging deep-space probe. They take apart its real ARM flight binary running on emulated hardware, uplink commands through a ground station with realistic signal delays, and decode the telemetry that comes back. New missions are authored as content, not code, so the sponsor, educators, and the wider community can keep building realistic training for the engineers who will maintain tomorrow's spacecraft.

## Getting Started

- Run `docker compose up` in the root of the repository
    - To pull the image instead compiling, run: `docker compose -f docker-compose.yml -f docker-compose.source.yml up`
- Visit `localhost:8080`
- Add custom scenarios at the mountpoint: `docker volume inspect sojourn_scenario_data --format '{{ .Mountpoint }}'`
