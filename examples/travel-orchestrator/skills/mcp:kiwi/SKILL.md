---
description: Flight search for Kiwi.com. Use search-flight to find one-way and return flights between two locations for given dates and passengers. Use feedback-to-devs only to forward feedback, bug reports, or feature requests about the search-flight tool itself to the developers — not for Claude/app behaviour, or other MCP servers.
name: mcp:kiwi
---
## Calling tools
Before calling `call_mcp_tool`, always read the selected tool's schema file: `schemas/<tool_name>.json`.
Strictly follow the schema to pass the correct arguments to the tool.

Use the **exact** tool name from the "Available tools" table below. Do not guess,
abbreviate, or transform names (for example, do not swap `-` for `_` or do not change case).
If `call_mcp_tool` returns "tool not found", re-read this file and use the exact name of the correct tool from the table.

## Available tools
| Tool name | Description |
|------|-------------|
| search-flight | # Search for flights  Searches Kiwi.com for available flights between two locations for the given dates and passengers. City or airport names are resolved automatically, so call this whenever the user wants to search for flights — whether they gave IATA codes or just place names.  ## Result shape  Returns `{ query, currency, passengers, resultsCount, itineraries, searchTimeMs }`. Each item in `itineraries` has: - `price` (number) and `priceFormatted` (e.g. "123 EUR") - `totalDurationSeconds` - `bookingUrl` — the link to book the flight - `imageId` — destination city id for a hero photo (https://images.kiwi.com/photos/600x600/{imageId}.jpg) - `baggage` — total included baggage across all travelers: `{ personalItem, cabinBag, checkedBag }` (counts) - `outbound` (and `inbound` for return flights), each a leg with: `route` (list of airport codes including layovers, e.g. ["PRG","MAD","BCN"]), `departureTime` / `arrivalTime` (local ISO timestamps), `durationSeconds`, `stops`, `cabinClass`, a... |
| feedback-to-devs | Send feedback, bug reports, or feature requests about the Kiwi.com search-flight MCP tool to its developers. This channel is ONLY for the flight search tool — search results, pricing, itineraries, filters, or errors in search responses. Do NOT use it for other MCP servers, Claude/app behaviour, account or booking management, voice mode, or unrelated Kiwi.com features; those won't reach the right team. Include error messages, logs, or context that would help the developers reproduce the issue. |
