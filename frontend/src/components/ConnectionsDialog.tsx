import { useEffect, useState } from 'react'
import { api, errText } from '../api'
import { isSerialMode, isUDPMode, MODE_GROUPS, MODE_LABELS, type ConnectionConfig, type ConnectionInfo, type Mode } from '../types'
import { IconPlus, IconRefresh, IconTrash } from './Icons'
import { Modal } from './Modal'

const BAUDS = [1200, 2400, 4800, 9600, 14400, 19200, 38400, 57600, 115200, 230400, 460800, 921600]

export const blankConnection = (): ConnectionConfig => ({
  id: '', name: '', mode: 'tcp', serialPort: '', baudRate: 9600, dataBits: 8, parity: 'N', stopBits: 1,
  host: '127.0.0.1', port: 502, timeoutMs: 1000, delayMs: 20,
})

interface Props {
  connections: ConnectionInfo[]
  initialId?: string
  onClose: () => void
}

export function ConnectionsDialog({ connections, initialId, onClose }: Props) {
  const [selId, setSelId] = useState<string>(initialId ?? connections[0]?.id ?? '')
  const [form, setForm] = useState<ConnectionConfig>(() => connections.find((c) => c.id === selId) ?? blankConnection())
  const [ports, setPorts] = useState<string[]>([])
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)
  const current = connections.find((c) => c.id === selId)

  const loadPorts = () => api.serialPorts().then(setPorts).catch((e) => setError(errText(e)))
  useEffect(() => { loadPorts() }, [])

  const select = (c?: ConnectionInfo) => {
    setSelId(c?.id ?? '')
    setForm(c ? { ...c } : blankConnection())
    setError('')
  }
  const set = <K extends keyof ConnectionConfig>(k: K, v: ConnectionConfig[K]) => setForm((f) => ({ ...f, [k]: v }))
  const isSerial = isSerialMode(form.mode)

  const save = async (connect: boolean) => {
    setBusy(true)
    setError('')
    try {
      const saved = await api.saveConnection(form)
      setSelId(saved.id)
      setForm({ ...saved })
      if (connect) {
        await api.connect(saved.id)
        onClose()
      }
    } catch (e) {
      setError(errText(e))
    } finally {
      setBusy(false)
    }
  }

  return (
    <Modal
      title="Connection setup"
      onClose={onClose}
      width={820}
      footer={
        <>
          {current && (current.connected ? (
            <button className="btn outline" onClick={() => api.disconnect(current.id)}>Disconnect</button>
          ) : null)}
          <span style={{ flex: 1 }} />
          <button className="btn outline" onClick={onClose}>Close</button>
          <button className="btn outline" disabled={busy} onClick={() => save(false)}>Save</button>
          <button className="btn primary" disabled={busy} onClick={() => save(true)}>Save &amp; Connect</button>
        </>
      }
    >
      <div className="conn-layout">
        <div>
          <div className="conn-list">
            {connections.map((c) => (
              <div key={c.id} className={`conn-item${c.id === selId ? ' sel' : ''}`} onClick={() => select(c)}>
                <span className={`dot${c.connected ? ' ok' : c.error ? ' err' : ''}`} />
                <div style={{ minWidth: 0 }}>
                  <div className="nm">{c.name}</div>
                  <div className="sm">{c.summary}</div>
                </div>
              </div>
            ))}
            <div className={`conn-item${selId === '' ? ' sel' : ''}`} onClick={() => select()}>
              <IconPlus /> <span>New connection</span>
            </div>
          </div>
          {current && (
            <button className="btn sm danger" style={{ marginTop: 8 }} onClick={() => { api.deleteConnection(current.id); select() }}>
              <IconTrash /> Delete
            </button>
          )}
        </div>

        <div className="form">
          <label className="lbl">Connection type</label>
          <select
            value={form.mode}
            onChange={(e) => {
              const m = e.target.value as Mode
              // Modbus ASCII is usually 7 data bits, even parity.
              setForm((f) => ({ ...f, mode: m, port: f.port || 502, ...(m === 'ascii' && f.mode !== 'ascii' ? { dataBits: 7, parity: 'E' as const } : {}) }))
            }}
          >
            {MODE_GROUPS.map((g) => (
              <optgroup key={g.label} label={g.label}>
                {g.modes.map((m) => <option key={m} value={m}>{MODE_LABELS[m]}</option>)}
              </optgroup>
            ))}
          </select>
          <label className="lbl">Name</label>
          <input type="text" value={form.name} placeholder="(automatic)" onChange={(e) => set('name', e.target.value)} />

          {isSerial ? (
            <>
              <label className="lbl">Serial port</label>
              <div className="inline">
                <input type="text" list="ports" value={form.serialPort} placeholder={ports.length ? 'Select a port' : 'e.g. COM3 or /dev/ttyUSB0'} onChange={(e) => set('serialPort', e.target.value)} />
                <datalist id="ports">{ports.map((p) => <option key={p} value={p} />)}</datalist>
                <button className="btn outline icon" onClick={loadPorts} title="Refresh port list"><IconRefresh /></button>
              </div>
              {ports.length === 0 && <span className="hint">No serial ports detected; you can still type a port name.</span>}
              <label className="lbl">Baud rate</label>
              <select value={form.baudRate} onChange={(e) => set('baudRate', Number(e.target.value))}>
                {BAUDS.map((b) => <option key={b} value={b}>{b}</option>)}
              </select>
              <label className="lbl">Data bits / Parity / Stop</label>
              <div className="inline">
                <select value={form.dataBits} onChange={(e) => set('dataBits', Number(e.target.value))}>
                  <option value={8}>8 data bits</option><option value={7}>7 data bits</option>
                </select>
                <select value={form.parity} onChange={(e) => set('parity', e.target.value as ConnectionConfig['parity'])}>
                  <option value="N">None parity</option><option value="E">Even parity</option><option value="O">Odd parity</option>
                </select>
                <select value={form.stopBits} onChange={(e) => set('stopBits', Number(e.target.value))}>
                  <option value={1}>1 stop bit</option><option value={2}>2 stop bits</option>
                </select>
              </div>
            </>
          ) : (
            <>
              <label className="lbl">IP address / host</label>
              <input type="text" value={form.host} onChange={(e) => set('host', e.target.value)} />
              <label className="lbl">{isUDPMode(form.mode) ? 'UDP port' : 'TCP port'}</label>
              <input type="number" min={1} max={65535} value={form.port} onChange={(e) => set('port', Number(e.target.value))} />
            </>
          )}

          <label className="lbl">Response timeout</label>
          <div className="inline">
            <input type="number" min={10} value={form.timeoutMs} onChange={(e) => set('timeoutMs', Number(e.target.value))} /> <span className="muted">ms</span>
          </div>
          <label className="lbl">Delay between polls</label>
          <div className="inline">
            <input type="number" min={0} value={form.delayMs} onChange={(e) => set('delayMs', Number(e.target.value))} /> <span className="muted">ms</span>
          </div>
          {error && <div className="alert full">{error}</div>}
          {current?.error && !error && <div className="alert full">Last error: {current.error}</div>}
        </div>
      </div>
    </Modal>
  )
}
