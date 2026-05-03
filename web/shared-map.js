(function () {
  function formatKindLabel(kind) {
    return String(kind || '').replace(/_/g, ' ')
  }

  function pixelToHex(px, py, hexSize, sqrt3) {
    const r = Math.round(py / (hexSize * 1.5))
    const q = Math.round(px / (hexSize * sqrt3) - 0.5 * (r & 1))
    return { q, r }
  }

  function buildTileMap(mapData) {
    const tileMap = {}
    for (const tile of mapData?.tiles || []) {
      tileMap[`${tile.coord.q},${tile.coord.r}`] = tile
    }
    return tileMap
  }

  function buildEntityMap(state) {
    const entityMap = {}
    const add = (type, data) => {
      const key = `${data.position.q},${data.position.r}`
      ;(entityMap[key] = entityMap[key] || []).push({ type, data })
    }

    for (const unit of state?.team1?.units || []) add('unit', unit)
    for (const unit of state?.team2?.units || []) add('unit', unit)
    for (const building of state?.team1?.buildings || []) add('building', building)
    for (const building of state?.team2?.buildings || []) add('building', building)

    return entityMap
  }

  function buildContestedMap(state) {
    const contestedMap = {}
    for (const contest of state?.last_tick_contested_hexes || []) {
      contestedMap[`${contest.coord.q},${contest.coord.r}`] = contest
    }
    return contestedMap
  }

  function loadImage(images, src) {
    return new Promise((resolve) => {
      const img = new Image()
      img.onload = () => {
        images[src] = img
        resolve()
      }
      img.onerror = () => resolve()
      img.src = src
    })
  }

  async function fetchJSON(url) {
    try {
      const res = await fetch(url)
      if (!res.ok) return null
      return await res.json()
    } catch {
      return null
    }
  }

  function normalizeAppearance(teamData, teamNumber) {
    const fallback =
      teamNumber === 1
        ? { faction: 'linux', variant: 'blue' }
        : { faction: 'microsoft', variant: 'red' }
    const appearance = teamData?.appearance || {}
    return {
      faction: String(appearance.faction || fallback.faction).toLowerCase(),
      variant: String(appearance.variant || fallback.variant).toLowerCase(),
    }
  }

  function buildUnitAssetPath(teamData, unitKind, suffix = '.png') {
    const teamNumber = teamData?.units?.[0]?.team || teamData?.buildings?.[0]?.team || 0
    const appearance = normalizeAppearance(teamData, teamNumber)
    return `assets/units/${appearance.faction}/${unitKind}${suffix}`
  }

  function buildingStateFromHP(building) {
    if (!building.max_hp || building.max_hp <= 0) return 'idle'
    const ratio = building.hp / building.max_hp
    if (ratio <= 0.33) return 'burning'
    if (ratio <= 0.66) return 'damaged'
    return 'idle'
  }

  function buildBuildingAssetPath(teamData, buildingKind, suffix = '.png') {
    const teamNumber = teamData?.buildings?.[0]?.team || teamData?.units?.[0]?.team || 0
    const appearance = normalizeAppearance(teamData, teamNumber)
    return `assets/buildings/${appearance.faction}/${buildingKind}${suffix}`
  }

  function buildingSpritePath(faction, kind, state) {
    return `assets/sprites/buildings/${faction}/${kind}/${state}.png`
  }

  function variantColor(variant) {
    switch (String(variant || '').toLowerCase()) {
      case 'blue':
        return '#4a90e2'
      case 'red':
        return '#e25050'
      case 'green':
        return '#48a868'
      case 'gold':
      case 'yellow':
        return '#d6a93c'
      case 'purple':
        return '#8c62d8'
      default:
        return '#9aa7b6'
    }
  }

  async function preloadAssets(
    images,
    {
      terrains = [],
      unitKinds = [],
      buildingKinds = [],
      unitFactions = ['linux', 'microsoft'],
      buildingFactions = ['linux', 'microsoft'],
      spriteMeta = {},
    },
  ) {
    const tasks = []

    for (const terrain of terrains) {
      tasks.push(loadImage(images, `assets/sprites/terrain/${terrain}/atlas.png`))
    }

    // Sprite atlases (animated sprites in assets/sprites/{faction}/{kind}/)
    for (const faction of unitFactions) {
      for (const unitKind of unitKinds) {
        const metaUrl = `assets/sprites/${faction}/${unitKind}/atlas.meta.json`
        tasks.push(
          fetchJSON(metaUrl).then(async (meta) => {
            if (!meta) return
            spriteMeta[`${faction}/${unitKind}`] = meta
            await Promise.all([
              loadImage(images, meta.atlas.image),
              meta.atlas.mask_image
                ? loadImage(images, meta.atlas.mask_image)
                : Promise.resolve(),
            ])
          }),
        )
      }
    }

    // Old-style unit images as fallback (for units not yet migrated to atlas)
    for (const faction of unitFactions) {
      for (const unitKind of unitKinds) {
        tasks.push(loadImage(images, `assets/units/${faction}/${unitKind}.png`))
        tasks.push(loadImage(images, `assets/units/${faction}/${unitKind}_mask.png`))
      }
    }

    for (const faction of buildingFactions) {
      for (const buildingKind of buildingKinds) {
        for (const state of ['idle', 'damaged', 'burning']) {
          tasks.push(loadImage(images, buildingSpritePath(faction, buildingKind, state)))
        }
        // Legacy fallback paths (loaded silently; used if sprite path is missing).
        tasks.push(loadImage(images, `assets/buildings/${faction}/${buildingKind}.png`))
      }
    }

    await Promise.all(tasks)
  }

  // Returns the effective pixel scale from the current canvas transform.
  // Used to size scratch canvases at physical resolution (not logical units).
  function ctxPixelScale(ctx) {
    const t = ctx.getTransform()
    return Math.sqrt(t.a * t.a + t.b * t.b) || 1
  }

  // Creates a tinted copy of a mask image (or a sub-region of it) at physical resolution.
  // srcRect: optional { x, y, w, h } to extract a sub-region from the mask (for atlas frames).
  function makeMaskScratch(ctx, mask, sz, variantFill, srcRect = null) {
    const physicalSz = Math.ceil(sz * ctxPixelScale(ctx))
    const scratch = document.createElement('canvas')
    scratch.width = physicalSz
    scratch.height = physicalSz
    const scratchCtx = scratch.getContext('2d')
    scratchCtx.imageSmoothingEnabled = true
    scratchCtx.imageSmoothingQuality = 'high'
    if (srcRect) {
      scratchCtx.drawImage(mask, srcRect.x, srcRect.y, srcRect.w, srcRect.h, 0, 0, physicalSz, physicalSz)
    } else {
      scratchCtx.drawImage(mask, 0, 0, physicalSz, physicalSz)
    }
    scratchCtx.globalCompositeOperation = 'source-in'
    scratchCtx.fillStyle = variantFill
    scratchCtx.fillRect(0, 0, physicalSz, physicalSz)
    return scratch
  }

  // Maps unit.status to the atlas action key.
  // Backend emits uppercase status strings (e.g. "MOVING_FAST", "ATTACKING").
  function unitActionFromStatus(unit) {
    switch ((unit.status || '').toUpperCase()) {
      case 'MOVING_FAST':
      case 'MOVING_GUARD':
        return 'walk'
      case 'GATHERING':
      case 'BUILDING':
        return 'work'
      case 'ATTACKING':
        return 'attack'
      default:
        return 'idle'
    }
  }

  // Returns walk direction row index (0=down, 1=left, 2=right, 3=up).
  // moveDir: optional { dq, dr } representing the current animation step direction.
  // Falls back to status_target_coord if moveDir is not provided.
  function walkRow(unit, moveDir) {
    const dir = moveDir ||
      (unit.status_target_coord
        ? {
            dq: unit.status_target_coord.q - unit.position.q,
            dr: unit.status_target_coord.r - unit.position.r,
          }
        : null)
    if (!dir || (dir.dq === 0 && dir.dr === 0)) return 0
    if (Math.abs(dir.dr) >= Math.abs(dir.dq)) return dir.dr >= 0 ? 0 : 3
    return dir.dq > 0 ? 2 : 1
  }

  // Computes the source rect { x, y, w, h } to extract from the atlas for a given action + time.
  function actionFrameRect(action, actionName, unit, moveDir) {
    const now = performance.now()
    const fps = action.fps || 5
    const frameW = action.frame_width || 256
    const frameH = action.frame_height || 256

    let row, col
    if (actionName === 'walk' && action.row_semantics) {
      row = walkRow(unit, moveDir)
      col = Math.floor((now / 1000) * fps) % action.cols
    } else {
      const frameIdx = Math.floor((now / 1000) * fps) % action.frame_count
      row = Math.floor(frameIdx / action.cols)
      col = frameIdx % action.cols
    }

    return {
      x: action.rect.x + col * frameW,
      y: action.rect.y + row * frameH,
      w: frameW,
      h: frameH,
    }
  }

  // Draw a unit using its faction atlas with frame animation.
  // Falls back to drawVariantUnitSprite if no atlas is available for this faction/kind.
  // moveDir: optional { dq, dr } from the movement animation system for accurate walk direction.
  function drawAnimatedUnitSprite(ctx, images, spriteMeta, unit, teamData, cx, cy, size, moveDir = null) {
    const { faction } = normalizeAppearance(teamData, unit.team)
    const meta = spriteMeta[`${faction}/${unit.kind}`]

    if (!meta) {
      return drawVariantUnitSprite(ctx, images, unit, teamData, cx, cy, size)
    }

    const atlas = images[meta.atlas.image]
    if (!atlas) {
      return drawVariantUnitSprite(ctx, images, unit, teamData, cx, cy, size)
    }
    const atlasMask = images[meta.atlas.mask_image] || null

    const actionName = unitActionFromStatus(unit)
    const action = meta.actions[actionName] || meta.actions.idle
    if (!action) return false

    const srcRect = actionFrameRect(action, actionName, unit, moveDir)

    const sz = Math.ceil(size)
    // Foot-anchor: place the sprite so baseline_y aligns with the hex center (cy).
    const baselineY = meta.frame_spec?.baseline_y ?? 220
    const frameH = meta.frame_spec?.frame_height ?? 256
    const dx = Math.round(cx - sz / 2)
    const dy = Math.round(cy - (baselineY / frameH) * sz)

    ctx.save()
    ctx.drawImage(atlas, srcRect.x, srcRect.y, srcRect.w, srcRect.h, dx, dy, sz, sz)

    if (atlasMask) {
      const fill = variantColor(normalizeAppearance(teamData, unit.team).variant)
      ctx.drawImage(makeMaskScratch(ctx, atlasMask, sz, fill, srcRect), dx, dy, sz, sz)
    }

    ctx.restore()
    return true
  }

  // Building sprite sheet: 2048×2048, 2 cols × 2 rows of 1024×1024 frames at 4 fps.
  const BLDG_FRAME_SZ = 1024
  const BLDG_COLS = 2
  const BLDG_FPS = 4
  const BLDG_TOTAL_FRAMES = 4
  // Frame content at ~52-98% of frame; anchor building base to ~15px below hex centre.
  const BLDG_OFFSET_Y_RATIO = -0.38

  function drawVariantBuildingSprite(ctx, images, building, teamData, cx, cy, size) {
    const { faction } = normalizeAppearance(teamData, building.team)
    const state = buildingStateFromHP(building)

    const newImg =
      images[buildingSpritePath(faction, building.kind, state)] ||
      images[buildingSpritePath(faction, building.kind, 'idle')]
    const legacyImg = images[buildBuildingAssetPath(teamData, building.kind)]

    const sz = Math.ceil(size)
    const dx = Math.round(cx - sz / 2)
    const dy = Math.round(cy - sz / 2 + sz * BLDG_OFFSET_Y_RATIO)

    ctx.save()
    if (newImg) {
      const frameIdx = Math.floor((performance.now() / 1000) * BLDG_FPS) % BLDG_TOTAL_FRAMES
      const col = frameIdx % BLDG_COLS
      const row = Math.floor(frameIdx / BLDG_COLS)
      ctx.drawImage(newImg, col * BLDG_FRAME_SZ, row * BLDG_FRAME_SZ, BLDG_FRAME_SZ, BLDG_FRAME_SZ, dx, dy, sz, sz)
    } else if (legacyImg) {
      ctx.drawImage(legacyImg, dx, dy, sz, sz)
    } else {
      ctx.restore()
      return false
    }
    ctx.restore()
    return true
  }

  function drawVariantUnitSprite(ctx, images, unit, teamData, cx, cy, size, flipHorizontally = false) {
    const basePath = buildUnitAssetPath(teamData, unit.kind)
    const maskPath = buildUnitAssetPath(teamData, unit.kind, '_mask.png')
    const base = images[basePath]
    const mask = images[maskPath]
    if (!base) {
      return false
    }

    const sz = Math.ceil(size)
    let dx = Math.round(cx - sz / 2)
    let dy = Math.round(cy - sz / 2)

    ctx.save()
    if (flipHorizontally) {
      ctx.translate(Math.round(cx), Math.round(cy))
      ctx.scale(-1, 1)
      dx = Math.round(-sz / 2)
      dy = Math.round(-sz / 2)
    }

    ctx.drawImage(base, dx, dy, sz, sz)

    if (mask) {
      const fill = variantColor(normalizeAppearance(teamData, unit.team).variant)
      ctx.drawImage(makeMaskScratch(ctx, mask, sz, fill), dx, dy, sz, sz)
    }

    ctx.restore()
    return true
  }

  function drawResourceRemaining(ctx, cx, cy, tile) {
    if (!tile.remaining || tile.remaining <= 0) return
    ctx.save()
    ctx.fillStyle = 'rgba(12, 14, 18, 0.78)'
    ctx.fillRect(cx - 16, cy + 10, 32, 15)
    ctx.strokeStyle = 'rgba(255,255,255,0.18)'
    ctx.lineWidth = 0.8
    ctx.strokeRect(cx - 16, cy + 10, 32, 15)
    ctx.fillStyle = '#f4f7f8'
    ctx.font = 'bold 10px system-ui'
    ctx.textAlign = 'center'
    ctx.textBaseline = 'middle'
    ctx.fillText(String(tile.remaining), cx, cy + 17.5)
    ctx.restore()
  }

  function drawConstructionOverlay(ctx, cx, cy, building, hexSize) {
    if (building.complete) return
    const ratio =
      building.build_ticks_total > 0
        ? building.build_progress / building.build_ticks_total
        : 0
    const width = hexSize * 1.1
    const x = cx - width / 2
    const y = cy + hexSize * 0.62

    ctx.save()
    ctx.fillStyle = 'rgba(0, 0, 0, 0.58)'
    ctx.fillRect(x, y, width, 8)
    ctx.fillStyle = 'rgba(255, 208, 92, 0.92)'
    ctx.fillRect(x, y, width * ratio, 8)
    ctx.strokeStyle = 'rgba(255,255,255,0.18)'
    ctx.lineWidth = 1
    ctx.strokeRect(x, y, width, 8)
    ctx.fillStyle = '#fff3cf'
    ctx.font = 'bold 10px system-ui'
    ctx.textAlign = 'center'
    ctx.textBaseline = 'bottom'
    ctx.fillText(`${building.build_progress}/${building.build_ticks_total}`, cx, y - 2)
    ctx.restore()
  }

  function drawProductionOverlay(ctx, cx, cy, building) {
    if (building.production_queue_len <= 0) return
    ctx.save()
    ctx.fillStyle = 'rgba(12, 14, 18, 0.82)'
    ctx.fillRect(cx + 10, cy - 28, 30, 16)
    ctx.strokeStyle = 'rgba(255,255,255,0.18)'
    ctx.lineWidth = 0.8
    ctx.strokeRect(cx + 10, cy - 28, 30, 16)
    ctx.fillStyle = '#cde3ff'
    ctx.font = 'bold 10px system-ui'
    ctx.textAlign = 'center'
    ctx.textBaseline = 'middle'
    ctx.fillText(
      `${building.production_queue_len}:${building.production_ticks_remaining}`,
      cx + 25,
      cy - 20,
    )
    ctx.restore()
  }

  function drawCarryOverlay(ctx, cx, cy, unit) {
    if (!unit.carry_amount || unit.carry_amount <= 0) return
    ctx.save()
    ctx.fillStyle = 'rgba(12, 14, 18, 0.82)'
    ctx.fillRect(cx + 10, cy + 8, 28, 14)
    ctx.strokeStyle = 'rgba(255,255,255,0.18)'
    ctx.lineWidth = 0.8
    ctx.strokeRect(cx + 10, cy + 8, 28, 14)
    ctx.fillStyle = '#e5f1d0'
    ctx.font = 'bold 10px system-ui'
    ctx.textAlign = 'center'
    ctx.textBaseline = 'middle'
    ctx.fillText(`${unit.carry_amount}`, cx + 24, cy + 15)
    ctx.restore()
  }

  function drawAttackOverlay(ctx, cx, cy, unit, hexSize) {
    if (!unit.attack_target_id) return
    const pulse = 0.78 + 0.22 * Math.sin(performance.now() / 180)
    ctx.save()
    ctx.strokeStyle =
      unit.team === 1
        ? `rgba(110, 195, 255, ${pulse})`
        : `rgba(255, 122, 122, ${pulse})`
    ctx.lineWidth = 2.4
    ctx.beginPath()
    ctx.arc(cx, cy, hexSize * 0.63, 0, Math.PI * 2)
    ctx.stroke()
    ctx.fillStyle = 'rgba(255, 244, 194, 0.92)'
    ctx.font = 'bold 12px system-ui'
    ctx.textAlign = 'center'
    ctx.textBaseline = 'middle'
    ctx.fillText('!', cx, cy - hexSize * 0.74)
    ctx.restore()
  }

  function drawContestedOverlay(ctx, cx, cy, contest, hexSize) {
    if (!contest) return
    const pulse = 0.72 + 0.28 * Math.sin(performance.now() / 140)
    ctx.save()
    ctx.globalCompositeOperation = 'screen'

    const glow = ctx.createRadialGradient(cx, cy, hexSize * 0.1, cx, cy, hexSize * 1.05)
    glow.addColorStop(0, `rgba(255, 240, 160, ${0.38 * pulse})`)
    glow.addColorStop(0.55, `rgba(255, 124, 92, ${0.22 * pulse})`)
    glow.addColorStop(1, 'rgba(255, 124, 92, 0)')
    ctx.fillStyle = glow
    ctx.beginPath()
    ctx.arc(cx, cy, hexSize * 1.05, 0, Math.PI * 2)
    ctx.fill()

    ctx.strokeStyle = `rgba(255, 220, 138, ${0.72 * pulse})`
    ctx.lineWidth = 2.2
    ctx.beginPath()
    ctx.arc(cx, cy, hexSize * (0.44 + 0.05 * pulse), 0, Math.PI * 2)
    ctx.stroke()

    ctx.strokeStyle = `rgba(255, 96, 96, ${0.85 * pulse})`
    ctx.lineWidth = 2.6
    ctx.lineCap = 'round'
    ctx.beginPath()
    ctx.moveTo(cx - hexSize * 0.34, cy - hexSize * 0.34)
    ctx.lineTo(cx + hexSize * 0.34, cy + hexSize * 0.34)
    ctx.moveTo(cx + hexSize * 0.34, cy - hexSize * 0.34)
    ctx.lineTo(cx - hexSize * 0.34, cy + hexSize * 0.34)
    ctx.stroke()

    ctx.fillStyle = 'rgba(255, 246, 206, 0.95)'
    ctx.font = 'bold 11px system-ui'
    ctx.textAlign = 'center'
    ctx.textBaseline = 'middle'
    ctx.fillText('⚔', cx, cy - hexSize * 0.78)
    ctx.restore()
  }

  function buildTooltipHTML(q, r, tile, entities, contest) {
    let html = `<b>(${q}, ${r})</b> ${formatKindLabel(tile.terrain)}`
    if (tile.remaining > 0) {
      html += `<br>Remaining ${tile.remaining}`
    }
    if (contest) {
      const team1Count = contest.team1_unit_ids?.length || 0
      const team2Count = contest.team2_unit_ids?.length || 0
      html += `<br><b>Contested</b> T1 ${team1Count} vs T2 ${team2Count}`
    }
    if (!entities) {
      return html
    }

    for (const { type, data } of entities) {
      const label = type === 'unit' ? 'Unit' : 'Building'
      html += `<br><b>${label} #${data.id}</b> · Team ${data.team} · ${formatKindLabel(data.kind)} HP ${data.hp}/${data.max_hp}`
      if (type === 'unit') {
        if (data.carry_amount > 0) {
          html += `<br>&nbsp;&nbsp;Carry ${data.carry_amount} ${data.carry_resource}`
        }
        if (data.attack_target_id) {
          html += `<br>&nbsp;&nbsp;Attacking #${data.attack_target_id}`
        }
        continue
      }

      html += `<br>&nbsp;&nbsp;${data.complete ? 'Complete' : `Building ${data.build_progress}/${data.build_ticks_total}`}`
      if (data.production_queue_len > 0) {
        html += `<br>&nbsp;&nbsp;Queue ${data.production_queue_len}, next ${data.production_ticks_remaining}t`
      }
    }

    return html
  }

  function hideTooltip(tooltip) {
    tooltip.style.display = 'none'
  }

  function bindCanvasTooltip({
    canvas,
    tooltip,
    getScene,
    isBlocked = () => false,
    pad,
    hexSize,
    sqrt3,
    positionOffset = 14,
  }) {
    canvas.addEventListener('mousemove', (event) => {
      if (isBlocked()) return

      const scene = getScene()
      const mapData = scene?.mapData
      const tileMap = scene?.tileMap || {}
      const entityMap = scene?.entityMap || {}
      const contestedMap = scene?.contestedMap || {}
      const zoomScale = scene?.zoomScale || 1
      if (!mapData) {
        hideTooltip(tooltip)
        return
      }

      const rect = canvas.getBoundingClientRect()
      const px = (event.clientX - rect.left) / zoomScale - pad
      const py = (event.clientY - rect.top) / zoomScale - pad
      const { q, r } = pixelToHex(px, py, hexSize, sqrt3)
      const tile = tileMap[`${q},${r}`]
      const entities = entityMap[`${q},${r}`]
      const contest = contestedMap[`${q},${r}`]

      if (
        !tile ||
        q < 0 ||
        q >= (mapData?.width || 0) ||
        r < 0 ||
        r >= (mapData?.height || 0)
      ) {
        hideTooltip(tooltip)
        return
      }

      tooltip.innerHTML = buildTooltipHTML(q, r, tile, entities, contest)
      tooltip.style.display = 'block'
      tooltip.style.left = event.clientX + positionOffset + 'px'
      tooltip.style.top = event.clientY + positionOffset + 'px'
    })

    canvas.addEventListener('mouseleave', () => {
      hideTooltip(tooltip)
    })
  }

  // Terrain sprite sheet: 2048×2048, 2 cols × 2 rows of 1024×1024 frames at 4 fps.
  const TERRAIN_FRAME_SZ = 1024
  const TERRAIN_COLS = 2
  const TERRAIN_FPS = 4
  const TERRAIN_TOTAL_FRAMES = 4

  function drawTerrainSprite(ctx, images, kind, cx, cy, iw, ih) {
    const atlas = images[`assets/sprites/terrain/${kind}/atlas.png`]
    if (!atlas) return false
    const frameIdx = Math.floor((performance.now() / 1000) * TERRAIN_FPS) % TERRAIN_TOTAL_FRAMES
    const col = frameIdx % TERRAIN_COLS
    const row = Math.floor(frameIdx / TERRAIN_COLS)
    ctx.drawImage(atlas, col * TERRAIN_FRAME_SZ, row * TERRAIN_FRAME_SZ, TERRAIN_FRAME_SZ, TERRAIN_FRAME_SZ, cx - iw / 2, cy - ih / 2, iw, ih)
    return true
  }

  window.AODMapUI = {
    buildContestedMap,
    buildEntityMap,
    buildTileMap,
    bindCanvasTooltip,
    drawContestedOverlay,
    drawAttackOverlay,
    drawCarryOverlay,
    drawConstructionOverlay,
    drawProductionOverlay,
    drawResourceRemaining,
    buildingSpritePath,
    buildingStateFromHP,
    drawAnimatedUnitSprite,
    drawTerrainSprite,
    drawVariantBuildingSprite,
    drawVariantUnitSprite,
    buildBuildingAssetPath,
    buildUnitAssetPath,
    formatKindLabel,
    hideTooltip,
    normalizeAppearance,
    pixelToHex,
    preloadAssets,
    variantColor,
  }
})()
