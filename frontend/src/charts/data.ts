// Turns live buffers and recorded history into what uPlot draws.
export interface Points {
  t: number[] // unix ms, ascending
  v: (number | null)[]
}

const lowerBound = (a: number[], x: number) => {
  let lo = 0
  let hi = a.length
  while (lo < hi) {
    const mid = (lo + hi) >> 1
    if (a[mid] < x) lo = mid + 1
    else hi = mid
  }
  return lo
}

/** Points with from <= t <= to, plus one on each side so lines reach the edges. */
export function slice(p: Points, from: number, to: number): Points {
  const i = Math.max(0, lowerBound(p.t, from) - 1)
  const j = Math.min(p.t.length, lowerBound(p.t, to + 1) + 1)
  return { t: p.t.slice(i, j), v: p.v.slice(i, j) }
}

/** History first, then live points newer than the last recorded one. */
export function merge(hist: Points | undefined, liveP: Points | undefined, gapMs: number): Points {
  const out: Points = { t: [], v: [] }
  const push = (t: number, v: number | null) => {
    const last = out.t[out.t.length - 1]
    if (last !== undefined && t <= last) return
    if (last !== undefined && v !== null && t - last > gapMs && out.v[out.v.length - 1] !== null) {
      out.t.push(last + 1)
      out.v.push(null)
    }
    out.t.push(t)
    out.v.push(v)
  }
  if (hist) for (let i = 0; i < hist.t.length; i++) push(hist.t[i], hist.v[i])
  if (liveP) {
    const after = out.t.length ? out.t[out.t.length - 1] : -Infinity
    for (let i = lowerBound(liveP.t, after + 1); i < liveP.t.length; i++) push(liveP.t[i], liveP.v[i])
  }
  return out
}

/**
 * Min/max decimation into `buckets` time slots: keeps every peak while
 * drawing at most two points per slot. Gaps survive as null.
 */
export function decimate(p: Points, from: number, to: number, buckets: number): Points {
  if (p.t.length <= buckets * 2) return p
  const w = (to - from) / buckets
  const out: Points = { t: [], v: [] }
  let cur = -1
  let minT = 0, minV = Infinity, maxT = 0, maxV = -Infinity, gap = false, any = false
  const flush = () => {
    if (any) {
      if (minT === maxT) { out.t.push(minT); out.v.push(minV) }
      else if (minT < maxT) { out.t.push(minT, maxT); out.v.push(minV, maxV) }
      else { out.t.push(maxT, minT); out.v.push(maxV, minV) }
    }
    if (gap) {
      const last = out.t[out.t.length - 1]
      out.t.push(last === undefined ? from + cur * w : last + 1)
      out.v.push(null)
    }
    minV = Infinity; maxV = -Infinity; gap = false; any = false
  }
  for (let i = 0; i < p.t.length; i++) {
    const b = Math.floor((p.t[i] - from) / w)
    if (b !== cur) { flush(); cur = b }
    const v = p.v[i]
    if (v === null) { gap = true; continue }
    any = true
    if (v < minV) { minV = v; minT = p.t[i] }
    if (v > maxV) { maxV = v; maxT = p.t[i] }
  }
  flush()
  return out
}

export interface Stats { min: number; max: number; avg: number; last: number; n: number }

export function stats(p: Points, from: number, to: number): Stats | null {
  let min = Infinity, max = -Infinity, sum = 0, n = 0, last = NaN
  for (let i = lowerBound(p.t, from); i < p.t.length && p.t[i] <= to; i++) {
    const v = p.v[i]
    if (v === null) continue
    if (v < min) min = v
    if (v > max) max = v
    sum += v
    n++
    last = v
  }
  return n ? { min, max, avg: sum / n, last, n } : null
}

/** Formats a value with about `sig` significant digits, without float noise. */
export function fmtNum(v: number | null | undefined, sig = 6): string {
  if (v === null || v === undefined || !Number.isFinite(v)) return '—'
  if (v !== 0 && (Math.abs(v) >= 1e9 || Math.abs(v) < 1e-4)) return v.toExponential(3)
  const s = Number(v.toPrecision(sig))
  return s.toLocaleString(undefined, { maximumFractionDigits: 6 })
}

const pad = (n: number, w = 2) => String(n).padStart(w, '0')
/** Local time as YYYY-MM-DD HH:MM:SS.mmm for CSV exports. */
export const isoLocal = (ms: number) => {
  const d = new Date(ms)
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}:${pad(d.getSeconds())}.${pad(d.getMilliseconds(), 3)}`
}
