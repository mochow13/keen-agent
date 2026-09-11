---
description: Run the minimal trip intake before any transport, stay, or itinerary search.
name: trip-intake
---

# Trip intake

Activate when the user asks for a trip and the brief is incomplete.

## Procedure

1. Extract what is already given: destination, origin, dates/duration, party, budget, transport/stay leanings. Do not re-ask for these ($ARGUMENTS may carry the raw request).
2. Ask one batched round, max 9 questions: the 5 essentials (destination if missing, dates or duration + flexibility, origin + same-city return?, adults + children, budget tier: budget/mid-range/premium), then preferences only if they change the plan — transport/stay leaning, interests + pace (e.g. museums/history vs nature vs food, relaxed vs packed days), food/stay must-haves (dietary needs, cuisines to try/avoid, location/star/accessibility needs) — and always end with an open catch-all for additional requirements ("Anything else I should know — occasions, must-sees, things to avoid?").
3. Attach a default to each question ("Assume … unless you say otherwise") so a bare "just plan it" can proceed. Skip anything already given — never re-ask.
4. Emit a trip brief of 6 lines or fewer: destination, dates, route, party, budget, preferences + assumptions. Stop. Do not start searching until the user confirms or says to proceed on assumptions.
