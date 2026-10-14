import { useEffect, useMemo, useRef, useState, type ReactNode } from 'react'
import uPlot from 'uplot'
import 'uplot/dist/uPlot.min.css'
import { api, errText } from '../api'
import { IconPlus, IconX } from '../components/Icons'
import { decimate, fmtNum, isoLocal, merge, slice, stats, timeLabel, type Points, type Stats } from './data'
import { live } from './live'
import { HISTORY_FROM_SEC, RANGES, seriesColor, type PlotSeries } from './model'

type Theme = 'light' | 'dark'

// Chrome colours (text tokens, never series colours) per theme.
const INK = {
  light: { text: '#1c2430', muted: '#667085', grid: '#e4e8ee', panel: '#ffffff', cursor: '#98a2b3' },
  dark: { text: '#e3e8ef', muted: '#98a3b3', grid: '#262e38', panel: '#171c23', cursor: '#6b7686' },
}

interface Props {
  series: PlotSeries[]
  rangeSec: number
  onRange: (sec: number) => void
  theme: Theme
  title: string
  compact?: boolean
  onAxis?: (i: number) => void
  onRemove?: (i: number) => void
  onAdd?: () => void
  onError: (msg: string) => void
  onInfo: (msg: string) => void
  empty?: ReactNode
  toolbar?: ReactNode
}

const HIST_REFRESH_MS = 15_000

export function Chart({ series, rangeSec, onRange, theme, title, compact, onAxis, onRemove, onAdd, onError, onInfo, empty, toolbar }: Props) {
  const plotEl = useRef<HTMLDivElement>(null)
  const tipEl = useRef<HTMLDivElement | null>(null)
  const uRef = useRef<uPlot | null>(null)
  const zoomRef = useRef<[number, number] | null>(null)
  const histRef = useRef<Map<string, Points>>(new Map())
  const viewRef = useRef<{ from: number; to: number; pts: Points[] }>({ from: 0, to: 0, pts: [] })
  const seriesRef = useRef(series)
  seriesRef.current = series
  const [zoomed, setZoomed] = useState(false)
  const [st, setSt] = useState<(Stats | null)[]>([])
  const [ids, setIds] = useState<string[]>([])
  const history = rangeSec >= HISTORY_FROM_SEC

  // Rebuild the plot when what is drawn changes; data updates go through setData.
  const shape = series.map((s) => `${s.key}|${s.axis}|${s.color}|${s.step}|${s.label}|${s.unit}`).join(';')
  const refsSig = series.map((s) => (s.ref ? JSON.stringify(s.ref) : '')).join(';')

  // Recording IDs for history.
  useEffect(() => {
    if (!history) return
    const refs = series.map((s) => s.ref)
    if (!refs.some(Boolean)) { setIds([]); return }
    api.seriesIDs(refs.map((r) => r ?? { connection: '', slaveId: 0, function: 0, address: 0, format: '' }))
      .then((list) => setIds(list.map((id, i) => (refs[i] ? id : ''))))
      .catch(() => setIds([]))
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [refsSig, history])

  // Recorded history for long ranges, refreshed periodically.
  useEffect(() => {
    histRef.current = new Map()
    if (!history || ids.length === 0) return
    let stop = false
    const load = async () => {
      const to = Date.now()
      const from = to - rangeSec * 1000
      const next = new Map<string, Points>()
      await Promise.all(seriesRef.current.map(async (s, i) => {
        const id = ids[i]
        if (!id) return
        try {
          const samples = await api.querySeries(id, from, to, 1500)
          const gap = Math.max((rangeSec * 1000 / 1500) * 3, 10_000)
          next.set(s.key, merge({ t: samples.map((x) => x.t), v: samples.map((x) => x.v) }, undefined, gap))
        } catch (e) {
          if (!stop) onError(errText(e))
        }
      }))
      if (!stop) histRef.current = next
    }
    load()
    const t = setInterval(load, HIST_REFRESH_MS)
    return () => { stop = true; clearInterval(t) }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [ids.join(','), rangeSec, history])

  // Leaving zoom when the range changes.
  useEffect(() => { zoomRef.current = null; setZoomed(false) }, [rangeSec])

  useEffect(() => {
    const el = plotEl.current
    if (!el || series.length === 0) return
    const ink = INK[theme]
    const hasR = series.some((s) => s.axis === 'R')
    const bitsOnly = (axis: 'L' | 'R') => laneCount(series, axis) > 0
    const yRange = (axis: 'L' | 'R'): uPlot.Range.Function => (_u, min, max) => {
      if (bitsOnly(axis)) return [-0.3, laneCount(series, axis) * LANE - 0.2]
      if (min == null || max == null || !Number.isFinite(min)) return [0, 1]
      if (min === max) { const d = Math.abs(min) * 0.05 || 1; return [min - d, max + d] }
      const pad = (max - min) * 0.08
      return [min - pad, max + pad]
    }
    const axisFont = `${compact ? 10.5 : 11.5}px system-ui, -apple-system, "Segoe UI", Roboto, Ubuntu, sans-serif`
    const yAxis = (axis: 'L' | 'R'): uPlot.Axis => ({
      scale: axis,
      side: axis === 'L' ? 3 : 1,
      show: axis === 'L' || hasR,
      stroke: ink.muted,
      font: axisFont,
      labelFont: axisFont,
      size: compact ? 46 : 56,
      grid: { show: axis === 'L', stroke: ink.grid, width: 1 },
      ticks: { show: false },
      splits: bitsOnly(axis) ? () => Array.from({ length: laneCount(series, axis) }, (_, k) => [k * LANE, k * LANE + BIT_H]).flat() : undefined,
      values: bitsOnly(axis) ? (_u, vals) => vals.map((v) => (Math.round((v % LANE) * 10) === 0 ? '0' : '1')) : (_u, vals) => vals.map((v) => fmtNum(v, 5)),
    })
    const stepped = uPlot.paths.stepped!({ align: 1 })
    const opts: uPlot.Options = {
      width: Math.max(100, el.clientWidth),
      height: Math.max(80, el.clientHeight),
      legend: { show: false },
      padding: [20, hasR ? 4 : 14, 0, 4],
      cursor: {
        drag: { x: true, y: false, setScale: false },
        points: { size: 8, width: 2, fill: ink.panel },
        y: false,
      },
      select: { show: true, left: 0, top: 0, width: 0, height: 0 },
      scales: { x: { time: true }, L: { range: yRange('L') }, R: { range: yRange('R') } },
      axes: [
        {
          stroke: ink.muted, font: axisFont, grid: { stroke: ink.grid, width: 1 }, ticks: { show: false }, size: compact ? 26 : 30,
          values: (_u, vals, _ax, _space, incr) => vals.map((v) => timeLabel(v * 1000, incr)),
        },
        yAxis('L'),
        yAxis('R'),
      ],
      series: [
        {},
        ...series.map((s) => ({
          label: s.label,
          scale: s.axis,
          stroke: seriesColor(s.color, theme),
          width: 2,
          paths: s.step ? stepped : undefined,
          points: { show: false },
          spanGaps: false,
        })),
      ],
      hooks: {
        setSelect: [(u) => {
          if (u.select.width > 4) {
            const a = u.posToVal(u.select.left, 'x') * 1000
            const b = u.posToVal(u.select.left + u.select.width, 'x') * 1000
            zoomRef.current = [a, b]
            setZoomed(true)
            draw()
          }
          u.setSelect({ left: 0, top: 0, width: 0, height: 0 }, false)
        }],
        setCursor: [(u) => tooltip(u)],
      },
    }
    const u = new uPlot(opts, [[], ...series.map(() => [])], el)
    uRef.current = u
    u.over.addEventListener('dblclick', resetZoom)
    const tip = document.createElement('div')
    tip.className = 'chart-tip'
    tip.style.display = 'none'
    u.over.appendChild(tip)
    tipEl.current = tip
    const ro = new ResizeObserver(() => u.setSize({ width: Math.max(100, el.clientWidth), height: Math.max(80, el.clientHeight) }))
    ro.observe(el)
    draw()
    return () => {
      ro.disconnect()
      tipEl.current = null
      u.destroy()
      uRef.current = null
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [shape, theme, compact])

  function resetZoom() {
    zoomRef.current = null
    setZoomed(false)
    draw()
  }

  // Collects visible points and hands them to uPlot.
  function draw() {
    const u = uRef.current
    if (!u) return
    const now = Date.now()
    const [from, to] = zoomRef.current ?? [now - rangeSec * 1000, now]
    const buckets = Math.max(100, Math.floor(u.bbox.width / (devicePixelRatio || 1)))
    const pts: Points[] = []
    const tables: uPlot.AlignedData[] = []
    for (const s of seriesRef.current) {
      const b = live.get(s.key)
      const p = slice(merge(histRef.current.get(s.key), b, b?.gapMs ?? 10_000), from, to)
      pts.push(p)
      const d = decimate(p, from, to, buckets)
      // Bits on an all-bit axis get a lane each, like a logic analyser.
      const lane = laneOf(seriesRef.current, s)
      tables.push([d.t.map((t) => t / 1000), lane < 0 ? d.v : d.v.map((v) => (v === null ? null : v * BIT_H + lane * LANE))])
    }
    viewRef.current = { from, to, pts }
    const data = tables.some((t) => t[0].length)
      ? uPlot.join(tables)
      : ([[], ...tables.map(() => [])] as unknown as uPlot.AlignedData)
    u.batch(() => {
      u.setData(data, false)
      u.setScale('x', { min: from / 1000, max: to / 1000 })
    })
  }

  // Redraw while visible; stats twice a second.
  useEffect(() => {
    let n = 0
    const t = setInterval(() => {
      if (document.hidden || !uRef.current) return
      draw()
      if (n++ % 2 === 0) {
        const { from, to, pts } = viewRef.current
        setSt(pts.map((p) => stats(p, from, to)))
      }
    }, 500)
    return () => clearInterval(t)
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [rangeSec, shape])

  function tooltip(u: uPlot) {
    const tip = tipEl.current
    if (!tip) return
    const idx = u.cursor.idx
    const left = u.cursor.left ?? -1
    if (idx == null || left < 0 || !u.data[0].length) { tip.style.display = 'none'; return }
    const t = u.data[0][idx] * 1000
    const ser = seriesRef.current
    const rows = ser.map((s, i) => {
      const col = u.data[i + 1] as (number | null | undefined)[]
      let v = col?.[idx]
      for (let d = 1; v === undefined && d < 200; d++) v = col[idx - d] ?? col[idx + d]
      const lane = laneOf(ser, s)
      if (lane >= 0 && typeof v === 'number') v = Math.round((v - lane * LANE) / BIT_H)
      return `<div class="tt-row"><span class="tt-sw" style="background:${seriesColor(s.color, theme)}"></span><span class="tt-l">${esc(s.label)}</span><b>${fmtNum(v)}</b><span class="tt-u">${esc(s.unit)}</span></div>`
    })
    tip.innerHTML = `<div class="tt-t">${new Date(t).toLocaleString()}</div>${rows.join('')}`
    tip.style.display = 'block'
    const w = tip.offsetWidth
    const ow = u.over.clientWidth
    tip.style.left = `${left + 14 + w > ow ? Math.max(0, left - w - 14) : left + 14}px`
    tip.style.top = `${Math.max(0, Math.min((u.cursor.top ?? 0) - 10, u.over.clientHeight - tip.offsetHeight))}px`
  }

  const exportPNG = async () => {
    const u = uRef.current
    if (!u) return
    const src = u.ctx.canvas
    const dpr = devicePixelRatio || 1
    const ink = INK[theme]
    const pad = 12 * dpr
    const rowH = 18 * dpr
    const out = document.createElement('canvas')
    out.width = src.width + pad * 2
    out.height = src.height + pad * 3 + rowH * (series.length + 1)
    const g = out.getContext('2d')!
    g.fillStyle = ink.panel
    g.fillRect(0, 0, out.width, out.height)
    g.fillStyle = ink.text
    g.font = `600 ${13 * dpr}px system-ui, sans-serif`
    g.textBaseline = 'middle'
    g.fillText(title, pad, pad + rowH / 2)
    g.drawImage(src, pad, pad + rowH)
    g.font = `${12 * dpr}px system-ui, sans-serif`
    series.forEach((s, i) => {
      const y = pad * 2 + rowH + src.height + rowH * i + rowH / 2
      g.strokeStyle = seriesColor(s.color, theme)
      g.lineWidth = 2 * dpr
      g.beginPath(); g.moveTo(pad, y); g.lineTo(pad + 18 * dpr, y); g.stroke()
      const sv = st[i]
      g.fillStyle = ink.text
      g.fillText(`${s.label}${s.unit ? ` (${s.unit})` : ''}${s.axis === 'R' ? ' · right axis' : ''}`, pad + 26 * dpr, y)
      if (sv) {
        g.fillStyle = ink.muted
        g.fillText(`min ${fmtNum(sv.min)}   max ${fmtNum(sv.max)}   avg ${fmtNum(sv.avg)}   last ${fmtNum(sv.last)}`, out.width / 2, y)
      }
    })
    const b64 = out.toDataURL('image/png').split(',')[1]
    try {
      const p = await api.saveExportFile('Save chart image', `${fileBase(title)}.png`, 'PNG image', '*.png', b64, true)
      if (p) onInfo(`Saved ${p}`)
    } catch (e) {
      onError(errText(e))
    }
  }

  const exportCSV = async () => {
    const { pts } = viewRef.current
    const tables = pts.map((p) => [p.t, p.v] as uPlot.AlignedData)
    if (!tables.some((t) => t[0].length)) return onError('Nothing to export in this time range')
    const data = uPlot.join(tables)
    const head = ['Time', ...series.map((s) => `${s.label}${s.unit ? ` (${s.unit})` : ''}`)]
    const lines = [head.map(csvCell).join(',')]
    for (let i = 0; i < data[0].length; i++) {
      const row = [isoLocal(data[0][i])]
      for (let j = 1; j < data.length; j++) {
        const v = data[j][i]
        row.push(v === null || v === undefined ? '' : String(Number((v as number).toPrecision(12))))
      }
      lines.push(row.join(','))
    }
    try {
      const p = await api.saveExportFile('Export chart data', `${fileBase(title)}.csv`, 'CSV (*.csv)', '*.csv', '﻿' + lines.join('\r\n') + '\r\n', false)
      if (p) onInfo(`Saved ${p} (${data[0].length} rows)`)
    } catch (e) {
      onError(errText(e))
    }
  }

  const visibleRanges = useMemo(() => (compact ? RANGES.filter((r) => r.sec <= 24 * 3600) : RANGES), [compact])

  return (
    <div className={`chart${compact ? ' compact' : ''}`}>
      <div className="chart-bar">
        {toolbar}
        <div className="seg" role="group" aria-label="Time range">
          {visibleRanges.map((r) => (
            <button
              key={r.sec}
              className={r.sec === rangeSec ? 'on' : ''}
              title={r.sec >= HISTORY_FROM_SEC ? `Last ${r.label} (includes recorded history)` : `Last ${r.label}`}
              onClick={() => onRange(r.sec)}
            >{r.label}</button>
          ))}
        </div>
        {zoomed && <button className="btn sm" onClick={resetZoom} title="Back to the live range (or double-click the chart)">Reset zoom</button>}
        <span style={{ flex: 1 }} />
        {onAdd && <button className="btn sm" onClick={onAdd}><IconPlus />Add series</button>}
        <button className="btn sm" disabled={!series.length} onClick={exportPNG} title="Save as PNG image">PNG</button>
        <button className="btn sm" disabled={!series.length} onClick={exportCSV} title="Export the visible data as CSV">CSV</button>
      </div>
      <div className="chart-plot" ref={plotEl}>
        {series.length === 0 && <div className="chart-empty">{empty ?? 'No series yet'}</div>}
        {series.length > 0 && <span className="ax-unit l">{axisUnits(series, 'L')}</span>}
        {series.some((s) => s.axis === 'R') && <span className="ax-unit r">{axisUnits(series, 'R')}</span>}
      </div>
      {series.length > 0 && (
        <div className="chart-legend-wrap">
        <table className="chart-legend">
          <thead>
            <tr>
              <th />
              <th className="l">Series</th>
              <th>Axis</th>
              <th>Min</th>
              <th>Max</th>
              <th>Avg</th>
              <th>Last</th>
              <th className="l">Unit</th>
              {onRemove && <th />}
            </tr>
          </thead>
          <tbody>
            {series.map((s, i) => (
              <tr key={s.key} className={s.missing ? 'missing' : ''}>
                <td><span className="sw" style={{ background: seriesColor(s.color, theme) }} /></td>
                <td className="l name" title={s.missing ? 'This register is no longer read by its window' : s.label}>{s.label}</td>
                <td>
                  {onAxis
                    ? <button className="axis-btn" onClick={() => onAxis(i)} title="Switch between the left and right axis">{s.axis === 'L' ? 'Left' : 'Right'}</button>
                    : s.axis === 'L' ? 'Left' : 'Right'}
                </td>
                <td>{fmtNum(st[i]?.min)}</td>
                <td>{fmtNum(st[i]?.max)}</td>
                <td>{fmtNum(st[i]?.avg, 5)}</td>
                <td><b>{fmtNum(st[i]?.last)}</b></td>
                <td className="l muted">{s.unit}</td>
                {onRemove && <td><button className="btn sm icon x" onClick={() => onRemove(i)} title="Remove from chart"><IconX /></button></td>}
              </tr>
            ))}
          </tbody>
        </table>
        </div>
      )}
    </div>
  )
}

// Lanes for bit series: each takes BIT_H of a LANE-high band.
const LANE = 1.5
const BIT_H = 1
const allBits = (series: PlotSeries[], axis: 'L' | 'R') => {
  const on = series.filter((s) => s.axis === axis)
  return on.length > 0 && on.every((s) => s.step)
}
const laneCount = (series: PlotSeries[], axis: 'L' | 'R') => (allBits(series, axis) ? series.filter((s) => s.axis === axis).length : 0)
/** Lane index (top lane = first series), or -1 when the axis also has numbers. */
function laneOf(series: PlotSeries[], s: PlotSeries): number {
  if (!allBits(series, s.axis)) return -1
  const on = series.filter((x) => x.axis === s.axis)
  return on.length - 1 - on.indexOf(s)
}

const axisUnits = (series: PlotSeries[], axis: 'L' | 'R') =>
  [...new Set(series.filter((s) => s.axis === axis).map((s) => s.unit).filter(Boolean))].join(' / ')

const esc = (s: string) => s.replace(/[&<>"]/g, (c) => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;' })[c]!)
const csvCell = (s: string) => (/[",\r\n]/.test(s) ? `"${s.replace(/"/g, '""')}"` : s)
const fileBase = (s: string) => s.replace(/[\\/:*?"<>|]+/g, '_').trim() || 'chart'
