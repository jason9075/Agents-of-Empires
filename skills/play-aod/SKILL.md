---
name: play-aod
description: Play Agents of Dynasties through its HTTP API when the user asks to "Play AoD", "play Agents of Dynasties", control team 1 or team 2, or run a 10-second RTS command loop. Use this skill to poll `/state` and `/commands`, submit actions to `/command`, and keep playing until the game ends or the user stops the match.
---

# Play AoD

Use this skill when the user wants the agent to actively play **Agents of Dynasties** rather than merely explain the game.

Default assumptions unless the user says otherwise:

- Base URL: `http://127.0.0.1:8080`
- Team: `1`
- Decision cadence: every `10` seconds
- Operate until `game_over=true` or the user interrupts

This skill is intentionally text-only. Do not assume Python, Go, `jq`, or other extra tooling is installed. Prefer plain shell plus `curl`. If `curl` is unavailable, use another built-in HTTP client available in the environment.

## Workflow

1. Confirm or infer the target server and team.
2. Read [game-api.md](references/game-api.md) if you need the endpoint shapes, unit costs, or building rules.
3. Fetch `/map` once at start, then refresh it periodically because depleted resource tiles can turn into `plain`.
4. Enter a control loop:
   - `GET /state`
   - `GET /commands`
   - Decide what new commands are needed for the next tick
   - `POST /command` only for actors that are idle, mis-tasked, or need a better overwrite
   - Wait about 10 seconds
5. Stop when the match is over or the user redirects you.

## Core Control Rules

- Respect `X-Team-ID: 1` or `2` on every team-scoped request.
- Treat AoD as a persistent-command RTS:
  one `GATHER`, `MOVE_*`, `ATTACK`, or `BUILD` command can remain effective across many ticks.
- Use `/commands` to avoid re-sending the same command in the same tick window.
- Prefer changing only actors that actually need new orders.
- Re-read `/state` after each tick and react to:
  `last_tick_failed_commands`, `last_tick_contested_hexes`, lost units, depleted resources, and visible enemies.

## Default Strategy

Use a simple, robust macro-first strategy unless the user requests a different style.

Opening:

- Keep all villagers busy.
- Assign villagers to the nearest useful resource mix:
  food first, then wood, then gold, then stone.
- Keep the Town Center producing villagers until you have around 6 to 8 villagers or population pressure changes priorities.

Midgame:

- Build exactly one military production building at a time.
- First default building: `barracks`.
- Add `archery_range` next if food, wood, and gold income are stable.
- Add `stable` later if economy is healthy and you want scouting or cavalry pressure.

Army production:

- `barracks`: favor `infantry` early; add `spearman` when enemy cavalry is visible.
- `archery_range`: produce `archer` when enemy infantry or spearmen are common.
- `stable`: produce `scout_cavalry` for scouting and map control; `paladin` only once economy is strong.

Combat:

- If visible enemy units are near your economy or buildings, defend first.
- Use `ATTACK` against visible nearby enemies rather than aimless movement.
- Use `MOVE_GUARD` for cautious advance and map control.
- Use `MOVE_FAST` for repositioning, scouting, or collapsing on exposed targets.

## Build Heuristics

- Only villagers can build.
- Only `plain` tiles are valid build targets.
- The target build tile itself must not already hold a unit or building.
- The builder only needs to become adjacent to the target tile; it does not stand on the building tile.
- Prefer build tiles adjacent to your Town Center or just behind your frontline, not isolated corners.
- If a build command fails due to blocking or invalid placement, pick another nearby `plain` tile.

## Resource Heuristics

- Keep at least 2 villagers on food in the opening.
- Keep enough wood income to support `barracks` and `archery_range`.
- Start gold gathering before committing to archers, spearmen, cavalry, or paladins.
- Add stone gathering before `barracks`, `archery_range`, or `stable` if current stockpile is insufficient.
- Refresh `/map` when a villager unexpectedly idles; the resource tile may have depleted.

## Decision Priorities Per Loop

Evaluate in this order:

1. Is the game over?
2. Did any queued command fail last tick?
3. Are any villagers idle?
4. Is the Town Center idle and can it afford a villager?
5. Do you need another production building?
6. Are military buildings idle and affordable to queue?
7. Are visible enemies threatening your economy or exposed buildings?
8. If stable and safe, should scouting or pressure commands be issued?

## Command Hygiene

- Avoid duplicate submissions for the same actor inside one unresolved tick.
- Last-command-wins applies, so overwriting a unit is allowed but should be deliberate.
- Do not spam `STOP` unless you specifically need to clear a bad persistent action.
- If a unit is already gathering the correct node or attacking the intended target, leave it alone.

## Practical Shell Pattern

Use commands of this shape when you need to play through the terminal:

```bash
curl -s http://127.0.0.1:8080/state -H 'X-Team-ID: 1'
curl -s http://127.0.0.1:8080/commands -H 'X-Team-ID: 1'
curl -s http://127.0.0.1:8080/command \
  -H 'Content-Type: application/json' \
  -H 'X-Team-ID: 1' \
  -d '{"unit_id":2,"kind":"GATHER","target_coord":{"q":5,"r":5}}'
sleep 10
```

When interacting live, summarize each loop briefly:

- current tick
- major economy state
- commands sent this loop
- visible threats
- next intent

## Failure Handling

- If the server is unreachable, verify the base URL and whether the game server is running.
- If a command returns `400`, inspect the error code and adjust the plan instead of retrying blindly.
- If a target enemy disappears from LOS, switch from `ATTACK` to scouting or defensive positioning.
- If resources are insufficient, shift more villagers to the limiting resource before retrying production or building.

## Reference

Read [game-api.md](references/game-api.md) when you need:

- endpoint request and response shapes
- legal command kinds
- unit and building kinds
- resource and combat reference
