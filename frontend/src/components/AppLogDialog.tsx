import { useEffect, useLayoutEffect, useMemo, useRef, useState } from 'react'
import { api, errText, events } from '../api'
import type { LogEntry, LogLevel } from '../types'
import { IconFolder, IconSave, IconTrash } from './Icons'
import { Modal } from './Modal'

const MAX = 5000

const SOURCE_LABEL: Record<string, string> = {
  app: 'App', connection: 'Connection', poll: 'Polling', write: 'Write', alarm: 'Alarm', simulator: 'Simulator',
  workspace: 'Workspace', update: 'Update', recording: 'Recording', scan: 'Scan', test: 'Test', ui: 'Message',
}
const LEVEL_LABEL: Record<LogLevel, string> = { info: 'Info', warn: 'Warning', error: 'Error' }
const RANK: Record<LogLevel, number> = { info: 0, warn: 1, error: 2 }

const pad = (n: number, w = 2) => String(n).padStart(w, '0')
const clock = (ms: number) => {
  const d = new Date(ms)
  return `${pad(d.getHours())}:${pad(d.getMinutes())}:${pad(d.getSeconds())}.${pad(d.getMilliseconds(), 3)}`
}

interface Props {
  onClose: () => void
  onInfo: (m: string) => void
}

/** Application log: what the app did and what went wrong, live. */
export function AppLogDialog({ onClose, onInfo }: Props) {
  const [entries, setEntries] = useState<LogEntry[]>([])
  const [minLevel, setMinLevel] = useState<LogLevel>('info')
  const [source, setSource] = useState('')
  const [search, setSearch] = useState('')
  const [follow, setFollow] = useState(true)
  const [folder, setFolder] = useState('')
  const [error, setError] = useState('')
  const bodyRef = useRef<HTMLDivElement>(null)

  useEffect(() => {
    let alive = true
    // Subscribe first so nothing logged while loading is lost; merge by id.
    const off = events.onAppLog((e) => setEntries((cur) => (cur.length && cur[cur.length - 1].id >= e.id ? cur : [...cur, e].slice(-MAX))))
    api.appLog(0).then((list) => alive && setEntries((cur) => {
      const last = list.length ? list[list.length - 1].id : 0
      return [...list, ...cur.filter((e) => e.id > last)].slice(-MAX)
    })).catch((e) => setError(errText(e)))
    api.appLogFolder().then(setFolder).catch(() => {})
    return () => { alive = false; off() }
  }, [])

  const sources = useMemo(() => [...new Set(entries.map((e) => e.source))].sort(), [entries])
  const shown = useMemo(() => {
    const words = search.toLowerCase().split(/\s+/).filter(Boolean)
    return entries.filter((e) => RANK[e.level] >= RANK[minLevel] && (!source || e.source === source)
      && words.every((w) => e.message.toLowerCase().includes(w)))
  }, [entries, minLevel, source, search])
  const counts = useMemo(() => ({
    warn: entries.filter((e) => e.level === 'warn').length,
    error: entries.filter((e) => e.level === 'error').length,
  }), [entries])

  useLayoutEffect(() => {
    if (follow && bodyRef.current) bodyRef.current.scrollTop = bodyRef.current.scrollHeight
  }, [shown, follow])

  const run = async (fn: () => Promise<void>) => {
    try {
      setError('')
      await fn()
    } catch (e) {
      setError(errText(e))
    }
  }

  return (
    <Modal
      title="Application log"
      onClose={onClose}
      width={1040}
      footer={
        <>
          <span className="muted applog-foot" title={folder}>
            {shown.length === entries.length ? `${entries.length} entries` : `${shown.length} of ${entries.length} entries`}
            {folder && ` · daily files kept 14 days in ${folder}`}
          </span>
          <span style={{ flex: 1 }} />
          <button className="btn primary" onClick={onClose}>Close</button>
        </>
      }
    >
      <div className="applog-bar">
        <div className="seg" role="radiogroup" aria-label="Level">
          {(['info', 'warn', 'error'] as LogLevel[]).map((l) => (
            <button key={l} className={minLevel === l ? 'on' : ''} onClick={() => setMinLevel(l)}>
              {l === 'info' ? 'All' : l === 'warn' ? `Warnings & errors${counts.warn + counts.error ? ` (${counts.warn + counts.error})` : ''}` : `Errors${counts.error ? ` (${counts.error})` : ''}`}
            </button>
          ))}
        </div>
        <select value={source} onChange={(e) => setSource(e.target.value)} aria-label="Source">
          <option value="">All sources</option>
          {sources.map((s) => <option key={s} value={s}>{SOURCE_LABEL[s] ?? s}</option>)}
        </select>
        <input type="search" placeholder="Search messages…" value={search} onChange={(e) => setSearch(e.target.value)} style={{ flex: 1, minWidth: 140 }} />
        <label className="check" title="Keep the newest entry in view">
          <input type="checkbox" checked={follow} onChange={(e) => setFollow(e.target.checked)} /> Follow
        </label>
        <button className="btn sm outline" onClick={() => run(async () => { const p = await api.exportAppLog(); if (p) onInfo(`Saved ${p}`) })}><IconSave />Export…</button>
        <button className="btn sm outline" onClick={() => run(api.openAppLogFolder)}><IconFolder />Open folder</button>
        <button className="btn sm" title="Empty this list (the daily files are kept)" onClick={() => run(async () => { await api.clearAppLog(); setEntries([]) })}><IconTrash />Clear</button>
      </div>
      {error && <div className="alert">{error}</div>}
      <div
        className="applog-body"
        ref={bodyRef}
        onScroll={(e) => {
          const el = e.currentTarget
          const atEnd = el.scrollHeight - el.scrollTop - el.clientHeight < 24
          if (atEnd !== follow) setFollow(atEnd)
        }}
      >
        {shown.length === 0 ? (
          <div className="muted applog-empty">{entries.length ? 'No entries match the filter.' : 'Nothing logged yet.'}</div>
        ) : (
          <table className="applog-tbl">
            <tbody>
              {shown.map((e) => (
                <tr key={e.id} className={`lv-${e.level}`}>
                  <td className="t mono" title={new Date(e.time).toLocaleString()}>{clock(e.time)}</td>
                  <td className="l"><span className={`lv lv-${e.level}`}>{LEVEL_LABEL[e.level]}</span></td>
                  <td className="s">{SOURCE_LABEL[e.source] ?? e.source}</td>
                  <td className="m">{e.message}</td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </div>
    </Modal>
  )
}
