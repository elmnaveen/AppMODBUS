import { Fragment, useEffect, useMemo, useState } from 'react'
import { api, errText } from '../api'
import type { DataSettings, SeriesInfo } from '../types'
import { IconFolder, IconRefresh, IconSave, IconTrash } from './Icons'
import { Modal } from './Modal'

const fmtBytes = (n: number) =>
  n >= 1 << 30 ? `${(n / (1 << 30)).toFixed(2)} GB` : n >= 1 << 20 ? `${(n / (1 << 20)).toFixed(1)} MB` : n >= 1024 ? `${Math.round(n / 1024)} KB` : `${n} B`
const fmtTime = (ms: number) => (ms ? new Date(ms).toLocaleString() : '—')
const FN = ['', 'Coil', 'Discrete', 'Holding', 'Input']

// <input type="datetime-local"> works in local time without a zone.
const toLocalInput = (ms: number) => {
  const d = new Date(ms)
  const p = (n: number) => String(n).padStart(2, '0')
  return `${d.getFullYear()}-${p(d.getMonth() + 1)}-${p(d.getDate())}T${p(d.getHours())}:${p(d.getMinutes())}`
}
const fromLocalInput = (s: string) => new Date(s).getTime()

type Confirm = { title: string; body: string; action: () => Promise<void> } | null

interface Props {
  onClose: () => void
  onInfo: (m: string) => void
}

/** Recording storage settings, browsing, export and clean-up. */
export function DataDialog({ onClose, onInfo }: Props) {
  const [settings, setSettings] = useState<DataSettings | null>(null)
  const [series, setSeries] = useState<SeriesInfo[]>([])
  const [sel, setSel] = useState<Set<string>>(new Set())
  const [retention, setRetention] = useState('30')
  const [maxGB, setMaxGB] = useState('2')
  const [range, setRange] = useState(() => {
    const now = Date.now()
    return { from: toLocalInput(now - 3600_000), to: toLocalInput(now + 60_000) }
  })
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)
  const [confirm, setConfirm] = useState<Confirm>(null)

  const load = async () => {
    const [d, s] = await Promise.all([api.dataSettings(), api.recordedSeries()])
    setSettings(d)
    setSeries(s)
    setRetention(String(d.retentionDays))
    setMaxGB(d.maxStorageMB ? String(+(d.maxStorageMB / 1024).toFixed(2)) : '0')
    setSel((cur) => new Set([...cur].filter((id) => s.some((x) => x.id === id))))
  }
  useEffect(() => { load() }, [])

  const groups = useMemo(() => {
    const m = new Map<string, SeriesInfo[]>()
    for (const s of series) {
      const k = `${s.connection} · slave ${s.slaveId}`
      m.set(k, [...(m.get(k) ?? []), s])
    }
    return [...m.entries()]
  }, [series])

  const run = async (fn: () => Promise<void>) => {
    setBusy(true)
    setError('')
    try {
      await fn()
    } catch (e) {
      setError(errText(e))
    } finally {
      setBusy(false)
    }
  }
  const toggle = (ids: string[], on: boolean) => setSel((cur) => {
    const next = new Set(cur)
    ids.forEach((id) => (on ? next.add(id) : next.delete(id)))
    return next
  })
  const preset = (ms: number | 'all') => {
    const now = Date.now()
    if (ms === 'all') {
      const first = Math.min(...series.filter((s) => s.first).map((s) => s.first), now)
      setRange({ from: toLocalInput(first), to: toLocalInput(now + 60_000) })
    } else setRange({ from: toLocalInput(now - ms), to: toLocalInput(now + 60_000) })
  }

  const saveLimits = () => run(async () => {
    const days = Number(retention)
    const gb = Number(maxGB)
    if (!Number.isInteger(days) || days < 0) throw new Error('Retention must be a whole number of days (0 = forever)')
    if (!Number.isFinite(gb) || gb < 0) throw new Error('Maximum size must be a number of GB (0 = no limit)')
    setSettings(await api.setDataLimits(days, Math.round(gb * 1024)))
    await load()
    onInfo('Data limits saved')
  })
  const doExport = (format: 'csv' | 'xlsx') => run(async () => {
    const r = await api.exportRecording([...sel], fromLocalInput(range.from), fromLocalInput(range.to), format)
    if (r.path) onInfo(`Exported ${r.rows} rows to ${r.path}`)
  })

  const used = settings?.usedBytes ?? 0
  const cap = (settings?.maxStorageMB ?? 0) * 1024 * 1024
  const pct = cap ? Math.min(100, (100 * used) / cap) : 0
  const selCount = sel.size

  return (
    <Modal
      title="Data management"
      onClose={onClose}
      width={1100}
      footer={
        <>
          <span className="muted" style={{ fontSize: 12 }}>
            Turn on <b>REC</b> in a poll window to record its values here.
          </span>
          <span style={{ flex: 1 }} />
          <button className="btn outline" onClick={onClose}>Close</button>
        </>
      }
    >
      <div className="data-top">
        <div className="section" style={{ margin: 0 }}>
          <h4>Storage</h4>
          <div className="sbody">
            <div className="form" style={{ gridTemplateColumns: '130px 1fr' }}>
              <label className="lbl">Folder</label>
              <div className="inline">
                <span className="mono path" title={settings?.folder}>{settings?.folder}</span>
                <button className="btn sm outline" onClick={() => run(async () => setSettings(await api.chooseRecordingsFolder()))}><IconFolder />Change…</button>
                {settings && settings.folder !== settings.defaultFolder && (
                  <button className="btn sm" onClick={() => run(async () => setSettings(await api.resetRecordingsFolder()))}>Default</button>
                )}
              </div>
              <span className="hint">New recordings go to this folder; existing ones stay where they are.</span>
              <label className="lbl">Used</label>
              <div>
                <div className="usage"><div style={{ width: `${pct}%` }} className={pct > 90 ? 'hot' : ''} /></div>
                <span className="muted" style={{ fontSize: 12 }}>
                  {fmtBytes(used)}{cap ? ` of ${fmtBytes(cap)}` : ' (no limit)'} · {series.length} series
                </span>
              </div>
            </div>
          </div>
        </div>
        <div className="section" style={{ margin: 0 }}>
          <h4>Limits</h4>
          <div className="sbody">
            <div className="form" style={{ gridTemplateColumns: '150px 1fr' }}>
              <label className="lbl">Keep data for</label>
              <div className="inline"><input type="number" min={0} value={retention} onChange={(e) => setRetention(e.target.value)} style={{ width: 100 }} /> <span className="muted">days (0 = forever)</span></div>
              <label className="lbl">Maximum size</label>
              <div className="inline"><input type="number" min={0} step={0.5} value={maxGB} onChange={(e) => setMaxGB(e.target.value)} style={{ width: 100 }} /> <span className="muted">GB (0 = no limit)</span></div>
              <span className="hint">When full, the oldest days are deleted first. Today's data is always kept.</span>
              <div className="full"><button className="btn sm primary" disabled={busy} onClick={saveLimits}><IconSave />Save limits</button></div>
            </div>
          </div>
        </div>
      </div>

      <div className="data-bar">
        <b>Recordings</b>
        <button className="btn sm icon" title="Refresh" onClick={() => run(load)}><IconRefresh /></button>
        <span style={{ flex: 1 }} />
        <span className="muted">From</span>
        <input type="datetime-local" value={range.from} onChange={(e) => setRange((r) => ({ ...r, from: e.target.value }))} />
        <span className="muted">to</span>
        <input type="datetime-local" value={range.to} onChange={(e) => setRange((r) => ({ ...r, to: e.target.value }))} />
        <div className="seg">
          <button onClick={() => preset(3600_000)}>1 h</button>
          <button onClick={() => preset(86_400_000)}>24 h</button>
          <button onClick={() => preset(7 * 86_400_000)}>7 d</button>
          <button onClick={() => preset('all')}>All</button>
        </div>
      </div>

      <div className="data-list">
        {series.length === 0 ? (
          <div className="muted" style={{ padding: 20 }}>No recordings yet. Press <b>REC</b> on a poll window to start recording.</div>
        ) : (
          <table className="tbl">
            <thead>
              <tr>
                <th style={{ width: 28 }}><input type="checkbox" checked={selCount === series.length} onChange={(e) => toggle(series.map((s) => s.id), e.target.checked)} /></th>
                <th>Register</th><th>Address</th><th>Unit</th><th>Samples</th><th>Size</th><th>First</th><th>Last</th>
              </tr>
            </thead>
            <tbody>
              {groups.map(([device, items]) => (
                <Fragment key={device}>
                  <tr className="grp-row">
                    <td><input type="checkbox" checked={items.every((s) => sel.has(s.id))} onChange={(e) => toggle(items.map((s) => s.id), e.target.checked)} /></td>
                    <td colSpan={7}><b>{device}</b> <span className="muted">· {items.length} series · {fmtBytes(items.reduce((a, s) => a + s.bytes, 0))}</span></td>
                  </tr>
                  {items.map((s) => (
                    <tr key={s.id} className={sel.has(s.id) ? 'sel' : ''} onClick={() => toggle([s.id], !sel.has(s.id))}>
                      <td><input type="checkbox" checked={sel.has(s.id)} readOnly /></td>
                      <td>{s.name || <span className="muted">(unnamed)</span>} <span className="muted" style={{ fontSize: 11 }}>{s.window}</span></td>
                      <td className="mono">{FN[s.function]} {s.address}</td>
                      <td>{s.unit}</td>
                      <td className="mono">{s.samples.toLocaleString()}</td>
                      <td className="mono">{fmtBytes(s.bytes)}</td>
                      <td>{fmtTime(s.first)}</td>
                      <td>{fmtTime(s.last)}</td>
                    </tr>
                  ))}
                </Fragment>
              ))}
            </tbody>
          </table>
        )}
      </div>

      <div className="data-actions">
        <span className="muted">{selCount} selected</span>
        <span style={{ flex: 1 }} />
        <button className="btn sm outline" disabled={!selCount || busy} onClick={() => doExport('csv')}>Export CSV…</button>
        <button className="btn sm outline" disabled={!selCount || busy} onClick={() => doExport('xlsx')}>Export Excel…</button>
        <button
          className="btn sm danger"
          disabled={busy || !series.length}
          onClick={() => setConfirm({
            title: 'Delete time range',
            body: `Delete all samples from ${new Date(fromLocalInput(range.from)).toLocaleString()} to ${new Date(fromLocalInput(range.to)).toLocaleString()} in ${selCount ? `${selCount} selected series` : 'all series'}?`,
            action: async () => { await api.deleteRecordedRange([...sel], fromLocalInput(range.from), fromLocalInput(range.to)); await load() },
          })}
        >
          <IconTrash />Delete range{selCount ? '' : ' (all)'}
        </button>
        <button
          className="btn sm danger"
          disabled={!selCount || busy}
          onClick={() => setConfirm({
            title: 'Delete series',
            body: `Permanently delete ${selCount} series and all their recorded data?`,
            action: async () => { await api.deleteRecordedSeries([...sel]); setSel(new Set()); await load() },
          })}
        >
          <IconTrash />Delete series
        </button>
      </div>
      {error && <div className="alert" style={{ marginTop: 8 }}>{error}</div>}
      {settings?.error && !error && <div className="alert" style={{ marginTop: 8 }}>{settings.error}</div>}

      {confirm && (
        <Modal
          title={confirm.title}
          onClose={() => setConfirm(null)}
          width={460}
          footer={
            <>
              <button className="btn outline" onClick={() => setConfirm(null)}>Cancel</button>
              <button className="btn primary danger-fill" onClick={() => { const c = confirm; setConfirm(null); run(c.action) }}>Delete</button>
            </>
          }
        >
          <p style={{ margin: 0 }}>{confirm.body}</p>
          <p className="muted" style={{ margin: '8px 0 0' }}>This cannot be undone.</p>
        </Modal>
      )}
    </Modal>
  )
}
