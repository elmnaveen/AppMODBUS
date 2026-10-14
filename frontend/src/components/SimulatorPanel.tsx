import { useEffect, useState } from 'react'
import { api, errText } from '../api'
import { MODE_LABELS, type Mode, type SimConfig, type SimStatus, type SimTable } from '../types'
import { IconPlay, IconStop, IconX } from './Icons'

export const defaultSim: SimConfig = {
  mode: 'tcp', serialPort: '', baudRate: 9600, dataBits: 8, parity: 'N', stopBits: 1,
  host: '0.0.0.0', port: 5020, slaveId: 1, animate: true,
}

const TABLES: { id: SimTable; label: string }[] = [
  { id: 'coils', label: 'Coils 0x' },
  { id: 'discrete', label: 'Discrete 1x' },
  { id: 'input', label: 'Input 3x' },
  { id: 'holding', label: 'Holding 4x' },
]
const COUNT = 40

interface Props {
  onClose: () => void
  onStatus: (s: SimStatus) => void
}

export function SimulatorPanel({ onClose, onStatus }: Props) {
  const [cfg, setCfg] = useState<SimConfig>(defaultSim)
  const [status, setStatus] = useState<SimStatus | null>(null)
  const [error, setError] = useState('')
  const [table, setTable] = useState<SimTable>('holding')
  const [start, setStart] = useState(0)
  const [values, setValues] = useState<number[]>([])
  const [editing, setEditing] = useState<{ i: number; text: string } | null>(null)
  const set = <K extends keyof SimConfig>(k: K, v: SimConfig[K]) => setCfg((c) => ({ ...c, [k]: v }))
  const running = status?.running ?? false
  const bits = table === 'coils' || table === 'discrete'

  // Refresh status and the visible block of data twice a second.
  useEffect(() => {
    let live = true
    const tick = async () => {
      const [st, vals] = await Promise.all([api.simStatus(), api.simRead(table, start, COUNT)])
      if (!live) return
      setStatus(st)
      onStatus(st)
      setValues(vals)
      if (st.running) setCfg(st.config)
    }
    tick()
    const t = setInterval(tick, 500)
    return () => { live = false; clearInterval(t) }
  }, [table, start, onStatus])

  const toggle = async () => {
    setError('')
    try {
      if (running) await api.simStop()
      else await api.simStart(cfg)
      const st = await api.simStatus()
      setStatus(st)
      onStatus(st)
    } catch (e) {
      setError(errText(e))
    }
  }

  const commit = (i: number, text: string) => {
    setEditing(null)
    const v = Number(text.trim().toLowerCase().startsWith('0x') ? parseInt(text, 16) : text)
    if (!Number.isInteger(v) || v < -32768 || v > 65535) return setError(`"${text}" is not a 16-bit value`)
    setError('')
    api.simWrite(table, start + i, [v & 0xffff])
    setValues((vals) => vals.map((x, k) => (k === i ? v & 0xffff : x)))
  }

  const isSerial = cfg.mode === 'rtu'
  return (
    <aside className="drawer">
      <div className="panel-head" style={{ padding: '8px 12px' }}>
        <h3>Slave simulator</h3>
        <span className={`pill${running ? ' run' : ''}`}>{running ? 'Running' : 'Stopped'}</span>
        <span style={{ flex: 1 }} />
        <button className="btn sm icon" onClick={onClose} title="Hide"><IconX /></button>
      </div>
      <div className="drawer-body">
        <div className="section">
          <h4>Server</h4>
          <div className="sbody">
            <div className="form" style={{ gridTemplateColumns: '110px 1fr' }}>
              <label className="lbl">Mode</label>
              <select disabled={running} value={cfg.mode} onChange={(e) => set('mode', e.target.value as Mode)}>
                {(Object.keys(MODE_LABELS) as Mode[]).map((m) => <option key={m} value={m}>{MODE_LABELS[m]}</option>)}
              </select>
              {isSerial ? (
                <>
                  <label className="lbl">Serial port</label>
                  <input type="text" disabled={running} value={cfg.serialPort} placeholder="COM4 or /dev/ttyUSB1" onChange={(e) => set('serialPort', e.target.value)} />
                  <label className="lbl">Baud / format</label>
                  <div className="inline">
                    <input type="number" disabled={running} value={cfg.baudRate} onChange={(e) => set('baudRate', Number(e.target.value))} />
                    <select disabled={running} value={cfg.parity} onChange={(e) => set('parity', e.target.value)}>
                      <option value="N">8N1</option><option value="E">8E1</option><option value="O">8O1</option>
                    </select>
                  </div>
                </>
              ) : (
                <>
                  <label className="lbl">Listen on</label>
                  <div className="inline">
                    <input type="text" disabled={running} value={cfg.host} onChange={(e) => set('host', e.target.value)} />
                    <input type="number" disabled={running} style={{ width: 100 }} value={cfg.port} onChange={(e) => set('port', Number(e.target.value))} />
                  </div>
                </>
              )}
              <label className="lbl">Slave ID</label>
              <input type="number" disabled={running} min={1} max={247} value={cfg.slaveId} onChange={(e) => set('slaveId', Number(e.target.value))} />
              <label className="check full" style={{ gridColumn: 2 }}>
                <input type="checkbox" disabled={running} checked={cfg.animate} onChange={(e) => set('animate', e.target.checked)} />
                Live demo values (IR 0–9, DI 0–7)
              </label>
              <div className="full inline">
                <button className={`btn ${running ? 'outline' : 'primary'}`} onClick={toggle}>
                  {running ? <><IconStop />Stop simulator</> : <><IconPlay />Start simulator</>}
                </button>
                {running && status && (
                  <span className="muted mono" style={{ fontSize: 12 }}>{status.requests} requests · {status.clients} client(s)</span>
                )}
              </div>
              {(error || status?.error) && <div className="alert full">{error || status?.error}</div>}
            </div>
          </div>
        </div>

        <div className="section">
          <h4>
            Data
            <span style={{ flex: 1 }} />
            <span className="muted" style={{ fontWeight: 400 }}>Start</span>
            <input type="number" min={0} max={65535 - COUNT} value={start} style={{ width: 90, height: 24 }} onChange={(e) => setStart(Math.max(0, Math.min(65535 - COUNT, Number(e.target.value))))} />
          </h4>
          <div className="sbody">
            <div className="seg" style={{ marginBottom: 10 }}>
              {TABLES.map((t) => (
                <button key={t.id} className={table === t.id ? 'on' : ''} onClick={() => setTable(t.id)}>{t.label}</button>
              ))}
            </div>
            <div className="simgrid">
              {values.map((v, i) => (
                <div className="simcell" key={start + i}>
                  <span className="a">{start + i}</span>
                  {bits ? (
                    <label className="check">
                      <input type="checkbox" checked={v !== 0} onChange={(e) => { api.simWrite(table, start + i, [e.target.checked ? 1 : 0]); setValues((vs) => vs.map((x, k) => (k === i ? (e.target.checked ? 1 : 0) : x))) }} />
                      <span className={`bitdot${v ? ' on' : ''}`} />
                    </label>
                  ) : (
                    <input
                      type="text"
                      value={editing?.i === i ? editing.text : String(v)}
                      onFocus={(e) => { setEditing({ i, text: String(v) }); e.target.select() }}
                      onChange={(e) => setEditing({ i, text: e.target.value })}
                      onBlur={() => editing?.i === i && commit(i, editing.text)}
                      onKeyDown={(e) => e.key === 'Enter' && (e.target as HTMLInputElement).blur()}
                    />
                  )}
                </div>
              ))}
            </div>
            <p className="muted" style={{ fontSize: 11.5, margin: '10px 0 0' }}>Values are raw unsigned 16-bit (type 0x… for hex). Edits are visible to pollers immediately.</p>
          </div>
        </div>
      </div>
    </aside>
  )
}
