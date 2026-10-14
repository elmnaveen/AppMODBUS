import { useEffect, useLayoutEffect, useRef, useState } from 'react'
import { api, errText, events } from '../api'
import type { TrafficEntry } from '../types'
import { IconPause, IconPlay, IconSave, IconTrash, IconX } from './Icons'

const MAX_ROWS = 2000

const ts = (ms: number) => {
  const d = new Date(ms)
  const p = (n: number, w = 2) => n.toString().padStart(w, '0')
  return `${p(d.getHours())}:${p(d.getMinutes())}:${p(d.getSeconds())}.${p(d.getMilliseconds(), 3)}`
}

interface Props {
  visible: boolean
  onClose: () => void
  onError: (m: string) => void
}

/** Communication traffic window. Always mounted so it keeps collecting. */
export function TrafficPanel({ visible, onClose, onError }: Props) {
  const [rows, setRows] = useState<TrafficEntry[]>([])
  const [paused, setPaused] = useState(false)
  const [follow, setFollow] = useState(true)
  const [source, setSource] = useState('')
  const body = useRef<HTMLDivElement>(null)

  useEffect(
    () => events.onTraffic((batch) => setRows((r) => {
      const next = r.concat(batch)
      return next.length > MAX_ROWS ? next.slice(next.length - MAX_ROWS) : next
    })),
    [],
  )
  useLayoutEffect(() => {
    if (follow && visible && body.current) body.current.scrollTop = body.current.scrollHeight
  }, [rows, follow, visible])

  const sources = Array.from(new Set(rows.map((r) => r.source)))
  const shown = source ? rows.filter((r) => r.source === source) : rows

  const togglePause = () => {
    api.setTrafficPaused(!paused)
    setPaused(!paused)
  }
  const save = async () => {
    const text = shown.map((r) => `${ts(r.time)}  ${r.source.padEnd(24)} ${r.dir}  ${r.hex}`).join('\n') + '\n'
    try {
      await api.saveTrafficLog(text)
    } catch (e) {
      onError(errText(e))
    }
  }

  if (!visible) return null
  return (
    <div className="traffic">
      <div className="panel-head">
        <h3>Communication traffic</h3>
        <button className={`btn sm${paused ? ' on' : ''}`} onClick={togglePause}>{paused ? <><IconPlay />Resume</> : <><IconPause />Pause</>}</button>
        <button className="btn sm" onClick={() => setRows([])}><IconTrash />Clear</button>
        <button className="btn sm" onClick={save}><IconSave />Save…</button>
        <select style={{ width: 200, height: 26 }} value={source} onChange={(e) => setSource(e.target.value)}>
          <option value="">All sources</option>
          {sources.map((s) => <option key={s} value={s}>{s}</option>)}
        </select>
        <label className="check" style={{ marginLeft: 8 }}>
          <input type="checkbox" checked={follow} onChange={(e) => setFollow(e.target.checked)} /> Auto-scroll
        </label>
        <span className="muted" style={{ marginLeft: 'auto', fontSize: 12 }}>{shown.length} frames</span>
        <button className="btn sm icon" onClick={onClose} title="Hide"><IconX /></button>
      </div>
      <div className="traffic-body" ref={body}>
        {shown.length === 0 && <div className="muted" style={{ padding: 10, fontFamily: 'var(--sans)' }}>No traffic yet. Start a poll window to see raw Tx/Rx frames here.</div>}
        {shown.map((r) => (
          <div className="trow" key={r.seq}>
            <span className="t">{ts(r.time)}</span>
            <span className="s">{r.source}</span>
            <span className={`d ${r.dir}`}>{r.dir}</span>
            <span className="n">{r.len}B</span>
            <span>{r.hex}</span>
          </div>
        ))}
      </div>
    </div>
  )
}
