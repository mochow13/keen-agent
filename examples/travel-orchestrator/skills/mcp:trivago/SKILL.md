---
description: Use this skill to interact with the `trivago` MCP server.
name: mcp:trivago
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
| trivago-accommodation-radius-search | Search live hotel and accommodation listings near specific coordinates — a landmark, address, venue, or neighborhood — aggregated via trivago's metasearch of major booking sites. Returns the same, filterable results as trivago-accommodation-search. Use when the user anchors their search to a place rather than a named destination (e.g. "near the stadium," "within walking distance of X").  Additional Information: {"knownInformation":{"currentYear":"2026","today":"2026-09-10"}} |
| trivago-accommodation-search | Search live hotel and accommodation listings by destination or point of interest, aggregated via trivago's metasearch of major booking sites. Returns results with price, rating, amenities, and a link per property. Supports date range, guest/room configuration, and filters for star rating, guest rating, and amenities. Use for destination- or place-name-based lodging search.  Additional Information: {"knownInformation":{"currentYear":"2026","today":"2026-09-10"}} |
| trivago-destination-price-trends | Forecast monthly hotel price trends for a destination — average, minimum, and maximum nightly rate by star rating, across a date window. Use for planning questions like "when is the cheapest month to visit X" or "how do prices change through the year" — this is a forecasting tool, not a live-availability search.  Additional Information: {"knownInformation":{"currentYear":"2026","today":"2026-09-10"}} |
