---
description: Use this skill to interact with the `exa` MCP server.
name: mcp:exa
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
| web_search_exa | Search the web for any topic and get clean, ready-to-use content.        Best for: Finding current information, news, facts, people, companies, or answering questions about any topic.       Returns: Clean text content from top search results.        Query tips:       describe the ideal page, not keywords. "blog post comparing React and Vue performance" not "React vs Vue".       Use category:people / category:company to search through Linkedin profiles / companies respectively.       If highlights are insufficient, follow up with web_fetch_exa on the best URLs. |
| web_search_advanced_exa | Advanced web search with full control over filters, domains, dates, and content options.  Best for: When you need specific filters like date ranges, domain restrictions, or category filters. Not recommended for: Simple searches - use web_search_exa instead. Returns: Search results with optional highlights, summaries, and subpage content. |
| web_fetch_exa | Read a webpage's full content as clean markdown. Use after web_search_exa when highlights are insufficient or to read any URL.  Best for: Extracting full content from known URLs. Batch multiple URLs in one call. Returns: Clean text content and metadata from the page(s). |
