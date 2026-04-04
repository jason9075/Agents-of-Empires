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

  async function preloadAssets(images, { terrains = [], unitKinds = [], buildingKinds = [] }) {
    const tasks = []
    for (const terrain of terrains) {
      tasks.push(loadImage(images, `assets/terrain/${terrain}.png`))
    }
    for (const team of [1, 2]) {
      for (const unitKind of unitKinds) {
        tasks.push(loadImage(images, `assets/units/team${team}/${unitKind}.png`))
      }
      for (const buildingKind of buildingKinds) {
        tasks.push(loadImage(images, `assets/buildings/team${team}/${buildingKind}.png`))
      }
    }
    await Promise.all(tasks)
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

  function buildTooltipHTML(q, r, tile, entities) {
    let html = `<b>(${q}, ${r})</b> ${formatKindLabel(tile.terrain)}`
    if (tile.remaining > 0) {
      html += `<br>Remaining ${tile.remaining}`
    }
    if (!entities) {
      return html
    }

    for (const { type, data } of entities) {
      html += `<br>#${data.id} · Team ${data.team} · ${formatKindLabel(data.kind)} HP ${data.hp}/${data.max_hp}`
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

      tooltip.innerHTML = buildTooltipHTML(q, r, tile, entities)
      tooltip.style.display = 'block'
      tooltip.style.left = event.clientX + positionOffset + 'px'
      tooltip.style.top = event.clientY + positionOffset + 'px'
    })

    canvas.addEventListener('mouseleave', () => {
      hideTooltip(tooltip)
    })
  }

  window.AODMapUI = {
    buildEntityMap,
    buildTileMap,
    bindCanvasTooltip,
    drawAttackOverlay,
    drawCarryOverlay,
    drawConstructionOverlay,
    drawProductionOverlay,
    drawResourceRemaining,
    formatKindLabel,
    hideTooltip,
    pixelToHex,
    preloadAssets,
  }
})()
