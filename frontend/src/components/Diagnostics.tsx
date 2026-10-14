import { useEffect, useRef, useState, type ReactNode } from 'react'
import uPlot from 'uplot'
import 'uplot/dist/uPlot.min.css'
import { api, errText, events } from '../api'
import { fmtNum, timeLabel } from '../charts/data'
import { seriesColor } from '../charts/model'
import type {
  ConnectionInfo, ConnectionStats, CounterValue, DeviceID, EchoResult, HistBin, IDScanRequest, MapRange, MapScanRequest,
  NetworkScanRequest, Poll, RTSample, ScanState,
} from '../types'
import { blankConnection } from './ConnectionsDialog'
import { IconChart, IconList, IconPlug, IconRefresh, IconServer, IconTag } from './Icons'
import { Modal } from './Modal'

type Section = 'stats' | 'ids' | 'map' | 'network' | 'device'
const FN_NAMES: Record<number, string> = { 1: '01 Coils', 2: '02 Discrete inputs', 3: '03 Holding registers', 4: '04 Input registers' }

interface Props {
  connections: ConnectionInfo[]
  theme: 'light' | 'dark'
  initialSection?: Section
  onPolls: (polls: Poll[]) => void
  onClose: () => void
  onInfo: (m: string) => void
}

/** Statistics, bus scanners and device tools (FC08 / FC43). */
export function DiagnosticsDialog({ connections, theme, initialSection, onPolls, onClose, onInfo }: Props) {
  const [section, setSection] = useState<Section>(initialSection ?? 'stats')
  // The backend keeps the latest scan; remember the last one of each kind.
  const [scans, setScans] = useState<Partial<Record<ScanState['kind'], ScanState>>>({})
  const [device, setDevice] = useState<{ connectionId: string; slaveId: number } | null>(null)

  useEffect(() => {
    // Ignore updates older than what is shown (the initial fetch can race events).
    const keep = (st: ScanState) => st.kind && setScans((cur) => ((cur[st.kind]?.seq ?? -1) > st.seq ? cur : { ...cur, [st.kind]: st }))
    api.scanState().then(keep)
    return events.onScan(keep)
  }, [])

  const nav: { id: Section; label: string; hint: string; icon: ReactNode }[] = [
    { id: 'stats', label: 'Statistics', hint: 'Requests, errors, response times', icon: <IconChart /> },
    { id: 'ids', label: 'Slave ID scan', hint: 'Which IDs answer on a connection', icon: <IconList /> },
    { id: 'map', label: 'Register map', hint: 'Which addresses a slave has', icon: <IconTag /> },
    { id: 'network', label: 'Network scan', hint: 'Modbus TCP devices on an IP range', icon: <IconServer /> },
    { id: 'device', label: 'Device tools', hint: 'Identification (43) and diagnostics (08)', icon: <IconPlug /> },
  ]
  const running = Object.values(scans).find((x) => x?.running)
  const busyOther = running && running.kind !== section ? running.kind : ''

  return (
    <Modal title="Diagnostics & scanning" onClose={onClose} width={1120}>
      <div className="conn-layout diag-layout">
        <div className="conn-list">
          {nav.map((n) => (
            <div key={n.id} className={`conn-item${section === n.id ? ' sel' : ''}`} onClick={() => setSection(n.id)}>
              {n.icon}
              <div>
                <div className="nm">{n.label}</div>
                <div className="sm diag-hint">{n.hint}</div>
              </div>
              {running?.kind === n.id && <span className="scan-dot" title="Scanning" />}
            </div>
          ))}
        </div>
        <div className="diag-main">
          {busyOther && (section === 'ids' || section === 'map' || section === 'network') && (
            <div className="alert">A {busyOther === 'ids' ? 'slave ID' : busyOther === 'map' ? 'register map' : 'network'} scan is running. Stop it before starting another one.</div>
          )}
          {section === 'stats' && <StatsSection connections={connections} theme={theme} />}
          {section === 'ids' && (
            <IDScanSection
              connections={connections}
              scan={scans.ids ?? null}
              busy={!!busyOther}
              onIdentify={(connectionId, slaveId) => { setDevice({ connectionId, slaveId }); setSection('device') }}
              onPolls={onPolls}
            />
          )}
          {section === 'map' && <MapScanSection connections={connections} scan={scans.map ?? null} busy={!!busyOther} onPolls={onPolls} />}
          {section === 'network' && <NetworkSection scan={scans.network ?? null} busy={!!busyOther} onInfo={onInfo} />}
          {section === 'device' && <DeviceSection connections={connections} initial={device} />}
        </div>
      </div>
    </Modal>
  )
}

// ---------- shared bits ----------

const pct = (a: number, b: number) => (b ? `${((100 * a) / b).toFixed(1)}%` : '—')
const ms = (v: number) => (v ? `${fmtNum(v, 3)} ms` : '—')

function useConnection(connections: ConnectionInfo[], initial?: string) {
  const [id, setId] = useState(initial && connections.some((c) => c.id === initial) ? initial : connections[0]?.id ?? '')
  useEffect(() => {
    if (!connections.some((c) => c.id === id)) setId(connections[0]?.id ?? '')
  }, [connections, id])
  return [id, setId] as const
}

function ConnectionSelect({ connections, value, onChange }: { connections: ConnectionInfo[]; value: string; onChange: (id: string) => void }) {
  if (connections.length === 0) return <span className="muted">No connections yet. Add one under Connection.</span>
  return (
    <select value={value} onChange={(e) => onChange(e.target.value)}>
      {connections.map((c) => <option key={c.id} value={c.id}>{c.name} · {c.summary}</option>)}
    </select>
  )
}

function NumField({ label, value, onChange, min, max, width = 90 }: { label: string; value: number; onChange: (v: number) => void; min?: number; max?: number; width?: number }) {
  return (
    <label className="diag-field">
      <span>{label}</span>
      <input type="number" value={Number.isFinite(value) ? value : ''} min={min} max={max} style={{ width }} onChange={(e) => onChange(e.target.valueAsNumber)} />
    </label>
  )
}

function Progress({ scan, label }: { scan: ScanState | null; label: string }) {
  if (!scan || (!scan.running && !scan.finished)) return null
  const p = scan.total ? Math.min(100, (100 * scan.done) / scan.total) : 0
  const secs = ((scan.finished || Date.now()) - scan.started) / 1000
  return (
    <div className="scan-progress">
      <div className="bar"><div style={{ width: `${p}%` }} /></div>
      <span className="muted">
        {scan.running ? `${label}${scan.phase ? ` · ${scan.phase}` : ''} · ${Math.floor(p)}%` : `Finished in ${secs.toFixed(1)} s`}
        {scan.error && <span className="err-text"> · {scan.error}</span>}
      </span>
    </div>
  )
}

function ScanButtons({ running, busy, disabled, onStart }: { running: boolean; busy: boolean; disabled?: boolean; onStart: () => void }) {
  return running
    ? <button className="btn outline" onClick={() => api.stopScan()}>Stop</button>
    : <button className="btn primary" disabled={busy || disabled} onClick={onStart}>Start scan</button>
}

// ---------- statistics ----------

function StatsSection({ connections, theme }: { connections: ConnectionInfo[]; theme: 'light' | 'dark' }) {
  const [sel, setSel] = useConnection(connections)
  const [data, setData] = useState<ConnectionStats[]>([])
  useEffect(() => {
    let stop = false
    const load = () => api.statistics(sel).then((d) => !stop && setData(d))
    load()
    const t = setInterval(load, 1000)
    return () => { stop = true; clearInterval(t) }
  }, [sel])
  const cur = data.find((c) => c.id === sel)
  if (connections.length === 0) return <p className="muted">Statistics appear once a connection has sent requests.</p>
  const s = cur?.stats
  return (
    <div className="diag-stack">
      <div className="diag-row">
        <label className="diag-field"><span>Connection</span><ConnectionSelect connections={connections} value={sel} onChange={setSel} /></label>
        <span style={{ flex: 1 }} />
        {s && <span className="muted">Since {new Date(s.since).toLocaleTimeString()}</span>}
        <button className="btn outline" onClick={() => api.resetStatistics(sel)}><IconRefresh />Reset</button>
      </div>
      {s && (
        <>
          <div className="kpis">
            <Kpi label="Requests" value={String(s.requests)} />
            <Kpi label="Successful" value={pct(s.ok, s.requests)} sub={`${s.ok} OK`} />
            <Kpi label="Timeouts" value={String(s.timeouts)} sub={pct(s.timeouts, s.requests)} />
            <Kpi label="CRC / invalid" value={String(s.crc + s.invalid)} sub={s.ioErrors ? `${s.ioErrors} I/O errors` : undefined} />
            <Kpi label="Exceptions" value={String(s.exceptions)} sub={Object.entries(s.byCode).map(([k, v]) => `${k}: ${v}`).join(' · ') || undefined} />
            <Kpi label="Response avg" value={ms(s.avgMs)} sub={`min ${ms(s.minMs)} · max ${ms(s.maxMs)}`} />
            <Kpi label="Response p95" value={ms(s.p95Ms)} sub={`median ${ms(s.p50Ms)}`} />
          </div>
          <div className="muted hint-line">
            Connection totals count every request: polls, writes, scans and device tools. Response times count answered requests only.
            {s.lastError && <> Last error: <span className="err-text">{s.lastError}</span></>}
          </div>
          <div className="diag-charts">
            <div className="diag-card">
              <div className="diag-card-h">Response time distribution <span className="muted">(last {s.recent?.length ?? 0} requests)</span></div>
              <Histogram bins={s.histogram ?? []} theme={theme} />
            </div>
            <div className="diag-card">
              <div className="diag-card-h">Response time over time</div>
              <RTChart samples={s.recent ?? []} theme={theme} />
            </div>
          </div>
          <table className="tbl stats-tbl">
            <thead>
              <tr><th>Poll window</th><th>Requests</th><th>OK</th><th>Timeouts</th><th>CRC/invalid</th><th>Exceptions</th><th>Avg</th><th>p95</th><th>Last error</th></tr>
            </thead>
            <tbody>
              {cur.polls.length === 0 && <tr><td colSpan={9} className="muted">No poll windows on this connection.</td></tr>}
              {cur.polls.map((p) => (
                <tr key={p.id}>
                  <td>{p.name}</td>
                  <td className="num">{p.stats.requests}</td>
                  <td className="num">{pct(p.stats.ok, p.stats.requests)}</td>
                  <td className="num">{p.stats.timeouts}</td>
                  <td className="num">{p.stats.crc + p.stats.invalid}</td>
                  <td className="num">{p.stats.exceptions}</td>
                  <td className="num">{ms(p.stats.avgMs)}</td>
                  <td className="num">{ms(p.stats.p95Ms)}</td>
                  <td className="err-text ell">{p.stats.lastError}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </>
      )}
    </div>
  )
}

function Kpi({ label, value, sub }: { label: string; value: string; sub?: string }) {
  return (
    <div className="kpi">
      <div className="kpi-l">{label}</div>
      <div className="kpi-v">{value}</div>
      {sub && <div className="kpi-s" title={sub}>{sub}</div>}
    </div>
  )
}

const binLabel = (b: HistBin) => (b.to ? `${b.from}–${b.to}` : `${b.from}+`)
const binTick = (b: HistBin) => (b.to ? String(b.from) : `${b.from}+`)

/** Single-series bar chart of response-time bins (ms). */
function Histogram({ bins, theme }: { bins: HistBin[]; theme: 'light' | 'dark' }) {
  const max = Math.max(1, ...bins.map((b) => b.count))
  const total = bins.reduce((a, b) => a + b.count, 0)
  if (!total) return <div className="diag-empty muted">No responses yet</div>
  return (
    <div className="hist">
      {bins.map((b) => (
        <div key={b.from} className="hist-col" title={`${binLabel(b)} ms: ${b.count} (${pct(b.count, total)})`}>
          <span className="hist-n">{b.count ? b.count : ''}</span>
          <div className="hist-bar" style={{ height: `${(b.count / max) * 100}%`, background: seriesColor(0, theme) }} />
          <span className="hist-l">{binTick(b)}</span>
        </div>
      ))}
      <span className="hist-unit muted">ms (bar = from this value to the next)</span>
    </div>
  )
}

/** Response time of recent requests (answered ones only). */
function RTChart({ samples, theme }: { samples: RTSample[]; theme: 'light' | 'dark' }) {
  const el = useRef<HTMLDivElement>(null)
  const u = useRef<uPlot | null>(null)
  useEffect(() => {
    const box = el.current
    if (!box) return
    const ink = theme === 'dark' ? { muted: '#98a3b3', grid: '#262e38' } : { muted: '#667085', grid: '#e4e8ee' }
    const font = '11px system-ui, -apple-system, "Segoe UI", Roboto, Ubuntu, sans-serif'
    u.current = new uPlot({
      width: Math.max(100, box.clientWidth),
      height: Math.max(80, box.clientHeight),
      legend: { show: false },
      cursor: { drag: { x: false, y: false }, y: false, points: { size: 7 } },
      scales: { x: { time: true }, y: { range: (_u, _min, max) => [0, (max || 1) * 1.15] } },
      axes: [
        { stroke: ink.muted, font, grid: { stroke: ink.grid, width: 1 }, ticks: { show: false }, size: 26, values: (_u, v, _a, _s, incr) => v.map((x) => timeLabel(x * 1000, incr)) },
        { stroke: ink.muted, font, grid: { stroke: ink.grid, width: 1 }, ticks: { show: false }, size: 46, values: (_u, v) => v.map((x) => fmtNum(x, 4)) },
      ],
      series: [{}, { label: 'Response time', stroke: seriesColor(0, theme), width: 2, points: { show: false } }],
    }, [[], []], box)
    const ro = new ResizeObserver(() => u.current?.setSize({ width: Math.max(100, box.clientWidth), height: Math.max(80, box.clientHeight) }))
    ro.observe(box)
    return () => { ro.disconnect(); u.current?.destroy(); u.current = null }
  }, [theme])
  useEffect(() => {
    const ok = samples.filter((s) => s.ok)
    u.current?.setData([ok.map((s) => s.t / 1000), ok.map((s) => s.ms)])
  }, [samples])
  return (
    <div className="rt-wrap">
      <span className="ax-unit l">ms</span>
      <div ref={el} className="rt-plot" />
      {!samples.some((s) => s.ok) && <div className="diag-empty muted rt-empty">No responses yet</div>}
    </div>
  )
}

// ---------- slave ID scan ----------

function IDScanSection({ connections, scan, busy, onIdentify, onPolls }: {
  connections: ConnectionInfo[]; scan: ScanState | null; busy: boolean
  onIdentify: (connectionId: string, slaveId: number) => void; onPolls: (p: Poll[]) => void
}) {
  const [conn, setConn] = useConnection(connections, scan?.connectionId)
  const [req, setReq] = useState<Omit<IDScanRequest, 'connectionId'>>({ from: 1, to: 247, function: 3, address: 0, timeoutMs: 200 })
  const [error, setError] = useState('')
  const set = <K extends keyof typeof req>(k: K, v: (typeof req)[K]) => setReq((r) => ({ ...r, [k]: v }))
  const c = connections.find((x) => x.id === conn)
  const estimate = Math.max(0, req.to - req.from + 1) * (req.timeoutMs + (c?.delayMs ?? 0)) / 1000
  const start = () => { setError(''); api.startIDScan({ ...req, connectionId: conn }).catch((e) => setError(errText(e))) }
  const open = async (slaveId: number) => {
    try {
      const p = await api.createPoll({
        id: '', name: `ID${slaveId} · ${FN_NAMES[req.function].slice(3)}`, connectionId: scan?.connectionId || conn, slaveId,
        function: req.function, address: req.address, quantity: 10, scanRateMs: 1000, format: 'signed',
        plcAddressing: false, cellFormats: {}, tags: {}, record: false,
      })
      onPolls([p])
    } catch (e) {
      setError(errText(e))
    }
  }
  return (
    <div className="diag-stack">
      <p className="muted diag-intro">Sends a one-value read to every slave ID and lists the ones that answer, including those that answer with an exception.</p>
      <div className="diag-row">
        <label className="diag-field"><span>Connection</span><ConnectionSelect connections={connections} value={conn} onChange={setConn} /></label>
      </div>
      <div className="diag-row">
        <NumField label="From ID" value={req.from} min={1} max={247} onChange={(v) => set('from', v)} width={70} />
        <NumField label="To ID" value={req.to} min={1} max={247} onChange={(v) => set('to', v)} width={70} />
        <label className="diag-field">
          <span>Probe with</span>
          <select value={req.function} onChange={(e) => set('function', Number(e.target.value))}>
            {[1, 2, 3, 4].map((f) => <option key={f} value={f}>{FN_NAMES[f]}</option>)}
          </select>
        </label>
        <NumField label="Address" value={req.address} min={0} max={65535} onChange={(v) => set('address', v)} width={80} />
        <NumField label="Timeout (ms)" value={req.timeoutMs} min={20} max={10000} onChange={(v) => set('timeoutMs', v)} width={80} />
        <span style={{ flex: 1 }} />
        <ScanButtons running={!!scan?.running} busy={busy} disabled={!conn} onStart={start} />
      </div>
      <div className="muted hint-line">Up to {estimate < 90 ? `${Math.ceil(estimate)} s` : `${Math.ceil(estimate / 60)} min`} when no slave answers. Over TCP gateways, "gateway target failed" answers count as no slave.</div>
      {error && <div className="alert">{error}</div>}
      <Progress scan={scan} label={`Scanning ${scan?.done ?? 0} of ${scan?.total ?? 0} IDs`} />
      {scan && (scan.ids.length > 0 || !scan.running) && (
        <table className="tbl res-tbl">
          <thead><tr><th>Slave ID</th><th>Answer</th><th className="num">Response</th><th /></tr></thead>
          <tbody>
            {scan.ids.length === 0 && <tr><td colSpan={4} className="muted">No slave answered.</td></tr>}
            {scan.ids.map((r) => (
              <tr key={r.slaveId}>
                <td className="mono"><b>{r.slaveId}</b></td>
                <td>{r.status === 'ok' ? 'Data' : <span className={r.status === 'garbled' ? 'err-text' : 'muted'}>{r.exception}</span>}</td>
                <td className="num">{ms(r.ms)}</td>
                <td className="acts">
                  <button className="btn sm outline" onClick={() => open(r.slaveId)}>Open poll window</button>
                  <button className="btn sm outline" onClick={() => onIdentify(scan.connectionId, r.slaveId)}>Identify</button>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      )}
    </div>
  )
}

// ---------- register map ----------

function MapScanSection({ connections, scan, busy, onPolls }: { connections: ConnectionInfo[]; scan: ScanState | null; busy: boolean; onPolls: (p: Poll[]) => void }) {
  const [conn, setConn] = useConnection(connections, scan?.connectionId)
  const [req, setReq] = useState<Omit<MapScanRequest, 'connectionId'>>({ slaveId: 1, functions: [1, 2, 3, 4], from: 0, to: 1999, timeoutMs: 500 })
  const [error, setError] = useState('')
  const [picked, setPicked] = useState<Set<string>>(new Set())
  const set = <K extends keyof typeof req>(k: K, v: (typeof req)[K]) => setReq((r) => ({ ...r, [k]: v }))
  const key = (r: MapRange) => `${r.function}:${r.from}`
  useEffect(() => { if (scan && !scan.running) setPicked(new Set(scan.ranges.map(key))) }, [scan?.running, scan?.ranges.length])
  const start = () => { setError(''); api.startMapScan({ ...req, connectionId: conn }).catch((e) => setError(errText(e))) }
  const create = async () => {
    if (!scan) return
    try {
      onPolls(await api.pollsFromMap(scan.connectionId, scan.slaveId, scan.ranges.filter((r) => picked.has(key(r)))))
    } catch (e) {
      setError(errText(e))
    }
  }
  const toggleFn = (f: number) => set('functions', req.functions.includes(f) ? req.functions.filter((x) => x !== f) : [...req.functions, f].sort())
  return (
    <div className="diag-stack">
      <p className="muted diag-intro">
        Reads each function in blocks of 100, re-reads rejected blocks in blocks of 10 and narrows partly readable ones down to single addresses.
        A short run that contains no multiple of 10 and touches no other readable address can be missed.
      </p>
      <div className="diag-row">
        <label className="diag-field"><span>Connection</span><ConnectionSelect connections={connections} value={conn} onChange={setConn} /></label>
        <NumField label="Slave ID" value={req.slaveId} min={0} max={255} onChange={(v) => set('slaveId', v)} width={70} />
      </div>
      <div className="diag-row">
        <span className="diag-field"><span>Functions</span>
          <span className="checks">
            {[1, 2, 3, 4].map((f) => (
              <label key={f}><input type="checkbox" checked={req.functions.includes(f)} onChange={() => toggleFn(f)} />{FN_NAMES[f]}</label>
            ))}
          </span>
        </span>
      </div>
      <div className="diag-row">
        <NumField label="From address" value={req.from} min={0} max={65535} onChange={(v) => set('from', v)} />
        <NumField label="To address" value={req.to} min={0} max={65535} onChange={(v) => set('to', v)} />
        <NumField label="Timeout (ms)" value={req.timeoutMs} min={20} max={10000} onChange={(v) => set('timeoutMs', v)} width={80} />
        <span style={{ flex: 1 }} />
        <ScanButtons running={!!scan?.running} busy={busy} disabled={!conn || req.functions.length === 0} onStart={start} />
      </div>
      <div className="muted hint-line">
        Up to {Math.ceil(Math.max(0, req.to - req.from + 1) / 100) * 21 * req.functions.length} requests where nothing is readable; readable areas need fewer.
      </div>
      {error && <div className="alert">{error}</div>}
      <Progress scan={scan} label="Probing" />
      {scan && !scan.running && scan.finished > 0 && (
        <>
          {scan.unsupported.length > 0 && (
            <div className="muted">Not supported (illegal function): {scan.unsupported.map((f) => FN_NAMES[f]).join(', ')}</div>
          )}
          <table className="tbl res-tbl">
            <thead><tr><th /><th>Function</th><th>Addresses</th><th className="num">Count</th></tr></thead>
            <tbody>
              {scan.ranges.length === 0 && <tr><td colSpan={4} className="muted">No readable addresses found in {req.from}–{req.to}.</td></tr>}
              {scan.ranges.map((r) => (
                <tr key={key(r)}>
                  <td><input type="checkbox" checked={picked.has(key(r))} onChange={() => setPicked((cur) => { const n = new Set(cur); n.has(key(r)) ? n.delete(key(r)) : n.add(key(r)); return n })} /></td>
                  <td>{FN_NAMES[r.function]}</td>
                  <td className="mono">{r.from === r.to ? r.from : `${r.from}–${r.to}`}</td>
                  <td className="num">{r.to - r.from + 1}</td>
                </tr>
              ))}
            </tbody>
          </table>
          {scan.ranges.length > 0 && (
            <div className="diag-row">
              <span className="muted">Slave {scan.slaveId}: {scan.ranges.length} range(s) found.</span>
              <span style={{ flex: 1 }} />
              <button className="btn primary" disabled={picked.size === 0} onClick={create}>Open {picked.size} as poll windows</button>
            </div>
          )}
        </>
      )}
    </div>
  )
}

// ---------- network scan ----------

function NetworkSection({ scan, busy, onInfo }: { scan: ScanState | null; busy: boolean; onInfo: (m: string) => void }) {
  const [req, setReq] = useState<NetworkScanRequest>({ range: '192.168.1.1-254', port: 502, units: '1', timeoutMs: 300, identify: true })
  const [error, setError] = useState('')
  const [added, setAdded] = useState<Set<string>>(new Set())
  const set = <K extends keyof NetworkScanRequest>(k: K, v: NetworkScanRequest[K]) => setReq((r) => ({ ...r, [k]: v }))
  const start = () => { setError(''); api.startNetworkScan(req).catch((e) => setError(errText(e))) }
  const add = async (h: ScanState['hosts'][number]) => {
    try {
      const name = h.identity?.length ? `${h.identity[0]} ${h.identity[1] ?? ''}`.trim() : h.host
      await api.saveConnection({ ...blankConnection(), name, mode: 'tcp', host: h.host, port: h.port })
      setAdded((s) => new Set(s).add(`${h.host}:${h.port}`))
      onInfo(`Added connection ${name}`)
    } catch (e) {
      setError(errText(e))
    }
  }
  return (
    <div className="diag-stack">
      <p className="muted diag-intro">Opens a TCP connection to every address in the range, then asks each open port for a holding register on the given unit IDs.</p>
      <div className="diag-row">
        <label className="diag-field"><span>Addresses</span>
          <input type="text" value={req.range} style={{ width: 230 }} onChange={(e) => set('range', e.target.value)} placeholder="192.168.1.1-254 or 10.0.0.0/24" />
        </label>
        <NumField label="Port" value={req.port} min={1} max={65535} onChange={(v) => set('port', v)} width={80} />
        <label className="diag-field"><span>Unit IDs</span>
          <input type="text" value={req.units} style={{ width: 90 }} onChange={(e) => set('units', e.target.value)} placeholder="1, 255" />
        </label>
        <NumField label="Connect timeout (ms)" value={req.timeoutMs} min={50} max={5000} onChange={(v) => set('timeoutMs', v)} width={80} />
      </div>
      <div className="diag-row">
        <label className="check"><input type="checkbox" checked={req.identify} onChange={(e) => set('identify', e.target.checked)} />Read device identification (FC43)</label>
        <span style={{ flex: 1 }} />
        <ScanButtons running={!!scan?.running} busy={busy} onStart={start} />
      </div>
      {error && <div className="alert">{error}</div>}
      <Progress scan={scan} label={`${scan?.done ?? 0} of ${scan?.total ?? 0} addresses`} />
      {scan && (scan.hosts.length > 0 || !scan.running) && (
        <table className="tbl res-tbl">
          <thead><tr><th>Address</th><th>Modbus</th><th>Unit IDs</th><th>Identification</th><th className="num">Connect</th><th /></tr></thead>
          <tbody>
            {scan.hosts.length === 0 && <tr><td colSpan={6} className="muted">No open port {req.port} found.</td></tr>}
            {scan.hosts.map((h) => (
              <tr key={`${h.host}:${h.port}`}>
                <td className="mono"><b>{h.host}</b>:{h.port}</td>
                <td>{h.status === 'modbus' ? 'Answers' : <span className="muted">Port open, no answer</span>}</td>
                <td className="mono">{h.units.join(', ') || '—'}</td>
                <td className="ell" title={h.identity?.join(' · ')}>{h.identity?.join(' · ') || <span className="muted">{h.exception || '—'}</span>}</td>
                <td className="num">{ms(h.ms)}</td>
                <td className="acts">
                  <button className="btn sm outline" disabled={added.has(`${h.host}:${h.port}`)} onClick={() => add(h)}>
                    {added.has(`${h.host}:${h.port}`) ? 'Added' : 'Add connection'}
                  </button>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      )}
    </div>
  )
}

// ---------- device tools ----------

function DeviceSection({ connections, initial }: { connections: ConnectionInfo[]; initial: { connectionId: string; slaveId: number } | null }) {
  const [conn, setConn] = useConnection(connections, initial?.connectionId)
  const [slave, setSlave] = useState(initial?.slaveId ?? 1)
  const [code, setCode] = useState(2)
  const [ident, setIdent] = useState<DeviceID | null>(null)
  const [echoCount, setEchoCount] = useState(10)
  const [echo, setEcho] = useState<EchoResult | null>(null)
  const [counters, setCounters] = useState<CounterValue[] | null>(null)
  const [busy, setBusy] = useState('')
  const [error, setError] = useState<Record<string, string>>({})
  const run = async (what: string, fn: () => Promise<void>) => {
    setBusy(what)
    setError((e) => ({ ...e, [what]: '' }))
    try {
      await fn()
    } catch (e) {
      setError((x) => ({ ...x, [what]: errText(e) }))
    } finally {
      setBusy('')
    }
  }
  useEffect(() => { setIdent(null); setEcho(null); setCounters(null) }, [conn, slave])
  return (
    <div className="diag-stack">
      <div className="diag-row">
        <label className="diag-field"><span>Connection</span><ConnectionSelect connections={connections} value={conn} onChange={setConn} /></label>
        <NumField label="Slave ID" value={slave} min={0} max={255} onChange={setSlave} width={70} />
      </div>
      <div className="diag-card">
        <div className="diag-card-h">Read device identification <span className="muted">(function 43 / 14)</span></div>
        <div className="diag-row">
          <select value={code} style={{ width: 'auto' }} onChange={(e) => setCode(Number(e.target.value))}>
            <option value={1}>Basic (vendor, product code, revision)</option>
            <option value={2}>Regular (adds URL, product and model name)</option>
            <option value={3}>Extended (adds private objects)</option>
          </select>
          <button className="btn outline" disabled={!conn || !!busy} onClick={() => run('id', async () => setIdent(await api.readDeviceIdentification(conn, slave, code)))}>
            {busy === 'id' ? 'Reading…' : 'Read'}
          </button>
        </div>
        {error.id && <div className="alert">{error.id}</div>}
        {ident && (
          <table className="tbl res-tbl">
            <tbody>
              {ident.objects.map((o) => <tr key={o.id}><td className="muted" style={{ width: 200 }}>{o.name}</td><td><b>{o.value}</b></td></tr>)}
              {ident.objects.length === 0 && <tr><td className="muted">The device returned no objects.</td></tr>}
            </tbody>
          </table>
        )}
      </div>
      <div className="diag-card">
        <div className="diag-card-h">Serial line diagnostics <span className="muted">(function 08; mostly supported by RTU devices)</span></div>
        <div className="diag-row">
          <NumField label="Echo requests" value={echoCount} min={1} max={1000} onChange={setEchoCount} width={70} />
          <button className="btn outline" disabled={!conn || !!busy} onClick={() => run('echo', async () => setEcho(await api.echoTest(conn, slave, echoCount)))}>
            {busy === 'echo' ? 'Testing…' : 'Run echo test'}
          </button>
          {echo && (
            <span className={echo.ok === echo.sent ? '' : 'err-text'}>
              {echo.ok}/{echo.sent} echoed{echo.ok ? ` · ${ms(echo.minMs)} / ${ms(echo.avgMs)} / ${ms(echo.maxMs)} (min/avg/max)` : ''}
            </span>
          )}
        </div>
        {echo && echo.errors.length > 0 && <div className="alert">{echo.errors.slice(0, 3).join(' · ')}</div>}
        {error.echo && <div className="alert">{error.echo}</div>}
        <div className="diag-row">
          <button className="btn outline" disabled={!conn || !!busy} onClick={() => run('cnt', async () => setCounters(await api.diagCounters(conn, slave)))}>
            {busy === 'cnt' ? 'Reading…' : 'Read counters'}
          </button>
          <button className="btn outline" disabled={!conn || !!busy} onClick={() => run('cnt', async () => { await api.clearDiagCounters(conn, slave); setCounters(await api.diagCounters(conn, slave)) })}>
            Clear counters
          </button>
        </div>
        {error.cnt && <div className="alert">{error.cnt}</div>}
        {counters && (
          <table className="tbl res-tbl">
            <tbody>
              {counters.map((c) => (
                <tr key={c.sub}>
                  <td className="mono muted" style={{ width: 60 }}>{c.sub.toString(16).toUpperCase().padStart(2, '0')}</td>
                  <td>{c.name}</td>
                  <td className="num">{c.value < 0 ? <span className="muted" title={c.error}>not supported</span> : <b>{c.value}</b>}</td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </div>
    </div>
  )
}
