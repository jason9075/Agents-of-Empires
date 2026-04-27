# Sprite Pipeline

This directory uses a fixed-cell sprite workflow.

## Goals

- Keep every frame in a predictable canvas size.
- Keep feet aligned to a shared baseline across all actions.
- Let AI generate artwork, but never let AI decide final sheet layout.
- Compose action sheets and atlases only after frames are normalized.

## Workflow

1. Define the unit spec before generating art.
2. Generate one action at a time.
3. Remove the chroma-key background from the raw action source.
4. Normalize base frames onto the fixed canvas.
5. Build an independent mask sheet for the same action.
6. Compose per-action sheets.
7. Pack per-action sheets into a unit atlas.
8. Emit metadata that only describes layout, anchors, and timing.

## Shared Rules

- AI output should target one action only, not a mixed-action atlas.
- Single-frame generation is allowed when consistency is weak, but per-action raw sheets are acceptable if cell boundaries are still recoverable.
- Every frame uses a fixed transparent canvas decided by the pipeline.
- Every frame shares the same:
  - camera angle
  - line weight
  - lighting direction
  - baseline
  - anchor convention
- Tintable cloth or trim areas are authored in cyan `#00e5ff`.
- Background is generated as a flat chroma-key color and removed locally.
- Base sheet and mask sheet must come from the same normalized frame layout.
- Never normalize `mask` as an independent sprite with its own trim box.
- Do not treat "extract cyan from the final painted sprite" as the production mask workflow.

## Frame Spec Tiers

Use a small set of standard frame sizes instead of custom sizes per unit.

### Humanoid

- `frame_width`: `256`
- `frame_height`: `256`
- `anchor_x`: `128`
- `baseline_y`: `220`
- Intended for: `villager`, `infantry`, `spearman`, `archer`

### Mounted

- `frame_width`: `320`
- `frame_height`: `256`
- `anchor_x`: `160`
- `baseline_y`: `220`
- Intended for: `scout_cavalry`, `paladin`

## Villager Prototype Spec

- Tier: `humanoid`
- View: `topdown_3_4`
- Anchor: `feet`
- Baseline: `y=220`
- Body height target: about `170-190px`
- Padding: keep at least `16px` clear on all sides

### Actions

- `idle`
  - `frames`: `4`
  - `layout`: `2x2`
  - `fps`: `5`
- `walk`
  - `frames`: `16`
  - `layout`: `4x4`
  - `fps`: `9`
  - `rows`: `down`, `left`, `right`, `up`
- `work`
  - `frames`: `6`
  - `layout`: `3x2`
  - `fps`: `8`
  - aliases: `gather`, `build`

## Normalize Step

For every generated action source:

1. Remove the chroma-key background.
2. Split the action into raw frame crops.
3. Trim each base crop.
4. Scale the subject to fit the target frame tier.
5. Align the subject's feet to `baseline_y`.
6. Center the subject on `anchor_x`.
7. Export normalized base frames.

The normalize step owns placement. Do not hand-tune final sheet positions by eye.

## Mask Step

The mask step must reuse the normalized base placement.

1. Start from the same per-frame crop order as the base action.
2. Build a dedicated mask frame for each normalized base frame.
3. Keep only the intended variant area:
   - cloth trim
   - sash
   - cape lining
   - team-color ribbons or heraldic fabric
4. Keep skin, hair, face, weapons, metal, leather, and wood outside the mask unless the art direction explicitly says otherwise.
5. Compose the action mask sheet from those already-aligned mask frames.

Preferred order of reliability:

1. best: generate or paint a dedicated mask sheet for the action
2. acceptable: derive the mask from the raw source using a narrow, repo-specific color-selection rule and then place it with the exact same offsets as the base frame
3. forbidden as final output: re-trim or re-normalize mask frames independently of base

Validation rule:

- Always create a red overlay preview from `atlas.png + atlas_mask.png` before accepting the result.
- If the red overlay hits face, exposed skin, axe head, boots, or empty background, the mask is wrong.

## Composition Rules

- Per-action sheets are deterministic grid compositions.
- All cells in a sheet use the same frame size.
- Atlases are deterministic packings of complete action sheets.
- Metadata must reference:
  - frame size
  - atlas rects
  - fps
  - anchors
  - baseline

## Naming

Per faction and unit:

- `idle.png`
- `idle_mask.png`
- `walk.png`
- `walk_mask.png`
- `work.png`
- `work_mask.png`
- `atlas.png`
- `atlas_mask.png`
- `atlas.meta.json`

Optional intermediate build folders are allowed under `/tmp`, but final project assets should stay clean.
