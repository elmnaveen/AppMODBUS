// In-memory history of every numeric cell, fed by poll updates, so a chart
// opened now already shows the last hour.
import type { Poll } from '../types'
import { liveKey } from './model'

const KEEP_MS = 3600_000
const MAX_POINTS = 40_000

export interface Buffer {
  t: number[] // unix ms
  v: (number | null)[] // null marks a gap (polling stopped or failed)
  format: string
  gapMs: number
}

const buffers = new Map<string, Buffer>()
const lastUpdate = new Map<string, number>()
let version = 0

export const live = {
  get: (key: string) => buffers.get(key),
  version: () => version,

  ingest(p: Poll) {
    const { config: c, state: s } = p
    if (!s.hasData || lastUpdate.get(c.id) === s.updatedAt) return
    lastUpdate.set(c.id, s.updatedAt)
    const t = s.updatedAt
    const gapMs = Math.max(3 * c.scanRateMs, 3000)
    for (const cell of s.cells) {
      if (cell.span === 0) continue
      const key = liveKey(c.id, cell.address)
      let b = buffers.get(key)
      if (!b || b.format !== cell.format) {
        // A different format means different numbers: start over.
        b = { t: [], v: [], format: cell.format, gapMs }
        buffers.set(key, b)
      }
      b.gapMs = gapMs
      const prev = b.t[b.t.length - 1]
      if (prev !== undefined && t <= prev) continue
      if (prev !== undefined && t - prev > gapMs && b.v[b.v.length - 1] !== null) {
        b.t.push(prev + 1)
        b.v.push(null)
      }
      b.t.push(t)
      b.v.push(cell.hasNum && Number.isFinite(cell.num) ? cell.num : null)
      trim(b, t)
    }
    version++
  },

  /** Forgets a closed poll window. */
  drop(pollId: string) {
    for (const k of buffers.keys()) if (k.startsWith(pollId + ':')) buffers.delete(k)
    lastUpdate.delete(pollId)
    version++
  },
}

function trim(b: Buffer, now: number) {
  let cut = 0
  while (cut < b.t.length && b.t[cut] < now - KEEP_MS) cut++
  cut = Math.max(cut, b.t.length - MAX_POINTS)
  if (cut > 256 || (cut > 0 && b.t.length >= MAX_POINTS)) {
    b.t.splice(0, cut)
    b.v.splice(0, cut)
  }
}
