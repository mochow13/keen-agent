---
description: HasData MCP server. Tools are generated from generated/endpoints.json.  This connection is limited to 2 of 63 HasData tools (apis=booking). Other HasData APIs exist but are not available here.
name: mcp:booking
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
| hasdata_booking_place_getBookingPlaceDetails | Get Booking Hotel Details  Fetches a single Booking.com property by its full URL for the given stay dates (`checkInDate` / `checkOutDate`) and guest composition (rooms, adults, children with ages). Returns the property identity (hotelId, title, address, coordinates), policies (free cancellation, no prepayment, child/pet stays), price, rating and review summary, photos, and the list of available room suites for the requested window. Use to enrich property listings with real-time availability and pricing, monitor a specific competitor hotel over time, validate amenities and photos before displaying venue details to end users, or fetch full details after discovering the property URL via the Booking Search endpoint. |
| hasdata_booking_search_getBookingSearchResults | Get Booking Search Results  Searches Booking.com for accommodations by destination keyword and stay dates (`checkInDate` / `checkOutDate`) with guest composition (rooms, adults, children with ages) and rich filtering: property type, star rating, review score, hotel and room facilities, distance from center, reservation policy, bed preference, travel group, online payment, accessibility, plus optional price range and bedroom/bathroom counts. Pagination is page-based with 25 results per page; locale is controlled by `language` and `currency`. Returns each hotel's `hotelId`, title and Booking URL, location info (city, address, coordinates, distance to center / nearest beach), policies (free cancellation, no prepayment, child/pet stays), price (per stay, before discount, discount, currency), rating, review summary and main photo. Use to power travel-planning agents, OTA price/inventory monitoring, hotel competitor analysis, lead-generation in the hospitality vertical, or to feed `hotelId` ... |
