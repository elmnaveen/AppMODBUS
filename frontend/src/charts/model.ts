// Chart definitions. They are owned by the UI and stored in the workspace
// as an opaque JSON document (see App.SetCharts).
import { isBitFunction, type ConnectionInfo, type Poll } from '../types'

export type Axis = 'L' | 'R'

/** One plotted register: a cell of a poll window. */
export interface ChartSeries {
  pollId: string
  address: number
  axis: Axis
  color: number // index into the palette, fixed when the series is added
}

/** A chart tab that can combine registers from any poll window. */
export interface ChartDef {
  id: string
  name: string
  rangeSec: number
  series: ChartSeries[]
}

/** The chart shown under a poll window's register table. */
export interface InlineChart {
  open: boolean
  rangeSec: number
  series: ChartSeries[]
}

export interface ChartsDoc {
  tabs: ChartDef[]
  inline: Record<string, InlineChart>
}

export const MAX_SERIES = 8
export const emptyCharts = (): ChartsDoc => ({ tabs: [], inline: {} })

/** Live ranges are drawn from memory; longer ones also read recordings. */
export const RANGES: { sec: number; label: string }[] = [
  { sec: 60, label: '1m' },
  { sec: 300, label: '5m' },
  { sec: 900, label: '15m' },
  { sec: 3600, label: '1h' },
  { sec: 6 * 3600, label: '6h' },
  { sec: 24 * 3600, label: '24h' },
  { sec: 7 * 86400, label: '7d' },
  { sec: 30 * 86400, label: '30d' },
]
export const HISTORY_FROM_SEC = 3600

// Categorical palette, fixed order (light and dark steps of the same hues).
export const PALETTE = {
  light: ['#2a78d6', '#eb6834', '#1baf7a', '#eda100', '#e87ba4', '#008300', '#6250d6', '#e34948'],
  dark: ['#3987e5', '#d95926', '#199e70', '#c98500', '#d55181', '#008300', '#9085e9', '#e66767'],
}
export const seriesColor = (i: number, theme: 'light' | 'dark') => PALETTE[theme][i % PALETTE[theme].length]

/** Accepts whatever the workspace stored (older files have no charts). */
export function parseCharts(raw: unknown): ChartsDoc {
  const doc = emptyCharts()
  if (!raw || typeof raw !== 'object' || Array.isArray(raw)) return doc
  const r = raw as Partial<ChartsDoc>
  const okSeries = (s: unknown): s is ChartSeries =>
    !!s && typeof (s as ChartSeries).pollId === 'string' && typeof (s as ChartSeries).address === 'number'
  const fixSeries = (list: unknown) =>
    (Array.isArray(list) ? list.filter(okSeries) : []).slice(0, MAX_SERIES).map((s, i) => ({
      pollId: s.pollId, address: s.address, axis: s.axis === 'R' ? 'R' as const : 'L' as const,
      color: Number.isInteger(s.color) ? s.color : i,
    }))
  if (Array.isArray(r.tabs)) {
    for (const t of r.tabs) {
      if (!t || typeof t.id !== 'string') continue
      doc.tabs.push({ id: t.id, name: String(t.name || 'Chart'), rangeSec: Number(t.rangeSec) || 300, series: fixSeries(t.series) })
    }
  }
  if (r.inline && typeof r.inline === 'object') {
    for (const [id, c] of Object.entries(r.inline)) {
      if (!c) continue
      doc.inline[id] = { open: !!c.open, rangeSec: Number(c.rangeSec) || 300, series: fixSeries(c.series) }
    }
  }
  return doc
}

/** Drops series (and inline charts) of poll windows that no longer exist. */
export function pruneCharts(doc: ChartsDoc, pollIds: Set<string>): ChartsDoc {
  const inline: Record<string, InlineChart> = {}
  for (const [id, c] of Object.entries(doc.inline)) if (pollIds.has(id)) inline[id] = c
  return {
    tabs: doc.tabs.map((t) => ({ ...t, series: t.series.filter((s) => pollIds.has(s.pollId)) })),
    inline,
  }
}

export const newChartId = () => `chart-${Date.now().toString(36)}${Math.random().toString(36).slice(2, 6)}`
export const isChartId = (id: string) => id.startsWith('chart-')

/** Lowest palette slot not used yet, so existing lines keep their colour. */
export function freeColor(series: ChartSeries[]): number {
  for (let i = 0; i < MAX_SERIES; i++) if (!series.some((s) => s.color === i)) return i
  return series.length
}

/**
 * Picks the axis for a new series: the axis already showing its unit, else
 * the right axis while it is unused, else the left one.
 */
export function pickAxis(series: ChartSeries[], unit: string, unitOf: (s: ChartSeries) => string): Axis {
  if (series.length === 0) return 'L'
  const same = series.find((s) => unitOf(s) === unit)
  if (same) return same.axis
  return series.some((s) => s.axis === 'R') ? 'L' : 'R'
}

export function addSeries(series: ChartSeries[], pollId: string, address: number, unitOf: (s: ChartSeries) => string, unit: string): ChartSeries[] {
  if (series.length >= MAX_SERIES || series.some((s) => s.pollId === pollId && s.address === address)) return series
  return [...series, { pollId, address, axis: pickAxis(series, unit, unitOf), color: freeColor(series) }]
}

/** A series with everything the plot needs to draw and label it. */
export interface PlotSeries {
  key: string // live buffer key
  label: string
  unit: string
  color: number
  axis: Axis
  step: boolean
  missing: boolean // the register is no longer part of the window
  ref?: { connection: string; slaveId: number; function: number; address: number; format: string }
}

export const liveKey = (pollId: string, address: number) => `${pollId}:${address}`

export function resolveSeries(series: ChartSeries[], polls: Record<string, Poll>, connections: ConnectionInfo[], withWindow: boolean): PlotSeries[] {
  return series.map((s) => {
    const p = polls[s.pollId]
    const cell = p?.state.cells.find((c) => c.address === s.address && c.span > 0)
    const base = p?.config.plcAddressing ? 1 : 0
    const name = cell?.name || `${p ? (p.config.function === 1 || p.config.function === 2 ? 'Bit' : 'Reg') : 'Reg'} ${s.address + base}`
    const conn = p && connections.find((c) => c.id === p.config.connectionId)
    return {
      key: liveKey(s.pollId, s.address),
      label: withWindow && p ? `${p.config.name} · ${name}` : name,
      unit: cell?.unit ?? '',
      color: s.color,
      axis: s.axis,
      step: p ? isBitFunction(p.config.function) : false,
      missing: !cell,
      ref: p && cell && conn
        ? { connection: conn.summary, slaveId: p.config.slaveId, function: p.config.function, address: s.address, format: cell.format }
        : undefined,
    }
  })
}

/** Numeric cells of a poll window that can be charted. */
export const chartableCells = (p: Poll) => p.state.cells.filter((c) => c.span > 0)

/** Default inline chart: the first few registers of the window. */
export function defaultInline(p: Poll): InlineChart {
  let series: ChartSeries[] = []
  const unitOf = (s: ChartSeries) => p.state.cells.find((c) => c.address === s.address)?.unit ?? ''
  for (const c of chartableCells(p).slice(0, 4)) series = addSeries(series, p.config.id, c.address, unitOf, c.unit ?? '')
  return { open: true, rangeSec: 300, series }
}
