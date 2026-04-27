---
name: generate2dsprite
description: "Generate and rebuild Agents-of-Dynasties sprite assets. Use for this repo's faction-based RTS unit sprites, masks, sheets, atlases, and sprite metadata under web/assets/sprites/."
---

# Generate2dsprite

Use this skill only for `Agents-of-Dynasties` sprite production.

Default assumption: the user wants RTS unit sprites that match this repo's faction system, fixed-cell pipeline, and `web/assets/sprites/` layout.

## Project Standard

Read [../../web/assets/sprites/PIPELINE.md](../../web/assets/sprites/PIPELINE.md) before generating or repacking unit sprites for this repo.

Treat that file as the source of truth for:

- frame tiers
- anchor and baseline conventions
- action defaults
- composition order
- atlas packaging
- mask validation rules

## Parameters

Infer these from the user request:

- `faction`: usually `linux` | `microsoft`
- `unit_kind`: `villager` | `infantry` | `spearman` | `archer` | `scout_cavalry` | `paladin`
- `action`: `idle` | `walk` | `work` | `attack`
- `bundle`: usually `unit_bundle`
- `frame_tier`: `humanoid` | `mounted` | explicit custom size
- `variant_strategy`: `mask_sheet` | `raw_color_select`
- `prompt`: faction art direction, silhouette notes, or rebuild constraints
- `name`: optional output slug when the user asks for a custom output name

If the request is ambiguous, keep the plan inside this repo's RTS unit scope:

- default asset type: grounded unit sprite action sheet
- default bundle: `unit_bundle`
- default actions: `villager -> idle/walk/work`, combat unit -> `idle/walk/attack`
- default sheet shapes: `idle=2x2`, `walk=4x4`, `work=3x2`, `attack=2x2`
- only expand to projectiles, impacts, or other non-unit assets when the user explicitly asks for them

## Agent Rules

- Decide the asset plan from this repo's current unit catalog and pipeline. Do not drift into a generic RPG asset planner.
- Write the art prompt yourself. Do not default to the prompt-builder script.
- Use built-in `image_gen` for raw image generation.
- Use local processing only for deterministic cleanup and packaging: chroma-key cleanup, mask extraction, frame splitting, alignment, normalization, sheet composition, atlas composition, and metadata emission.
- Never treat a model-generated full sprite sheet as final output if the cell size is unstable.
- Prefer generating one action at a time. When consistency risk is high, generate one frame at a time.
- Keep the solid chroma-key background rule unless the user explicitly wants a different workflow.
- Final metadata must describe the normalized output, not the raw generation dimensions.
- Base and mask must share the exact same normalized frame placement.
- Do not ship a mask that was independently trimmed or normalized from the base sheet.
- Do not use broad cyan color-picking on the final painted sprite as the default production mask workflow.
- Default output root is `web/assets/sprites/<faction>/<unit_kind>/`.
- Default metadata root is `web/assets/sprites/<unit_kind>.meta.json` plus per-faction `atlas.meta.json`.
- Unless the user explicitly asks otherwise, preserve the repo's Linux-vs-Microsoft medieval parody art direction.

## Workflow

### 1. Infer the asset plan

Pick the smallest useful output for this repo.

Examples:

- `linux villager sprites` -> `linux` + `villager` + `idle/walk/work`
- `microsoft infantry` -> `microsoft` + `infantry` + `idle/walk/attack`
- `redo linux scout_cavalry mask` -> rebuild only the mask sheets and atlas mask for `linux/scout_cavalry`
- `make all units` -> iterate the repo unit catalog and build the matching bundle for each unit kind

Do not switch to concepts like spell bundles, summon bundles, or general overworld NPC packs unless the user explicitly extends the repo in that direction.

Known repo unit defaults:

- `villager`
  - `idle`
  - `walk`
  - `work`
- all combat units
  - `idle`
  - `walk`
  - `attack`

### 2. Lock the frame spec before generation

Choose a standard frame tier first.

Project defaults from `web/assets/sprites/PIPELINE.md`:

- `humanoid`
  - `256x256`
  - `anchor_x=128`
  - `baseline_y=220`
- `mounted`
  - `320x256`
  - `anchor_x=160`
  - `baseline_y=220`

Do not start image generation until these are fixed:

- frame width
- frame height
- anchor convention
- baseline
- action set for the target unit kind
- frames per action
- grid layout per action
- output paths under `web/assets/sprites/`

### 3. Write prompts for normalized output

Keep the strict parts:

- solid chroma-key background
- exact action-only scope
- same character identity across frames
- same silhouette scale across frames
- explicit containment: nothing may cross the intended frame edge
- explicit baseline awareness for grounded actors
- no text, labels, UI, speech bubbles, or borders between cells
- exact grid count only
- full body stays inside each cell with visible foot clearance and magenta margin on all sides

For this repo, prefer prompts that describe:

- one action at a time
- consistent camera angle
- consistent outline weight
- enough empty margin for later normalization
- the repo's cute 2D RTS style, not generic pixel-art RPG style
- faction identity in the costume and silhouette, with variant color reserved for maskable cloth areas
- for walk cycles, 4 rows in `down/left/right/up` order with stable scale across all 16 frames
- for dedicated mask sheets, the same action and frame order as base, black background, white only on tintable cloth or heraldic trim

### 4. Generate raw art

Use built-in `image_gen`.

Preferred order:

1. generate one action sheet if the motion is simple and consistency is likely
2. if consistency drifts, switch to one frame at a time and compose locally
3. if the user asked for a rebuild, prefer recovering and reusing prior raw sources before generating fresh art

After generation:

- find the raw PNG under `$CODEX_HOME/generated_images/...`
- copy the selected image into a working folder
- keep the original generated image in place

### 5. Normalize locally

The normalize step owns placement and size.

For every action:

- remove the chroma-key background
- split the action into raw frame crops
- trim and isolate the subject per frame
- scale to fit the target tier
- align feet to the baseline for grounded actors
- center on the anchor
- export normalized base frames and the composed base action sheet

Do not hand-tune final sheet offsets by eye when the normalize step can do it deterministically.

### 6. Build mask sheets from the normalized base layout

Mask generation is a separate step after base placement is locked.

For every normalized base frame:

- keep the same crop order and the same placement offsets
- create a dedicated mask frame for tintable fabric or heraldic trim only
- exclude skin, face, hair, weapons, metal, leather, and background

Reliability order:

1. best: create a dedicated action mask sheet
2. acceptable: derive a narrow color-selection mask from the raw source, then place it with the exact same offsets as base
3. reject: independently trim or normalize mask frames

Before accepting output:

- build a red overlay preview from base + mask
- reject the result if overlay lands on skin, tools, or empty space

### 7. Compose sheets and atlas

- build one deterministic sheet per action
- use fixed grid cells for every frame in that action
- pack the completed action sheets into an atlas
- atlas layout must be deterministic and documented in metadata

### 8. Emit metadata

Metadata should describe:

- faction
- unit kind
- frame tier
- frame width and height
- anchor and baseline
- action frame counts
- grid layouts
- atlas rects
- fps
- aliases such as `work -> gather/build`

For this repo, store or update metadata under `web/assets/sprites/`.

## Defaults

Project defaults from the current pipeline:

- `idle`
  - humanoid actor -> `4` frames, `2x2`
- `walk`
  - topdown humanoid actor -> `16` frames, `4x4`
  - rows: `down`, `left`, `right`, `up`
- `work`
  - villager-style labor loop -> `6` frames, `3x2`
- `attack`
  - compact combat action -> prefer `4` frames, `2x2`
- use shared normalized size for every frame in an action
- use `anchor=feet` for grounded units
- use `variant_strategy=mask_sheet` by default

Repo output defaults:

- per-faction assets live under `web/assets/sprites/<faction>/<unit_kind>/`
- unit bundle meta lives under `web/assets/sprites/<unit_kind>.meta.json`

## Return Shape

For a fixed-cell unit bundle, expect:

- one folder per faction or variant
- normalized per-action sheets
- matching mask sheets
- an overlay preview in `/tmp` while validating masks
- atlas
- atlas mask
- action / atlas metadata

## Resources

- `../../web/assets/sprites/PIPELINE.md`: repo-specific sprite pipeline standard
