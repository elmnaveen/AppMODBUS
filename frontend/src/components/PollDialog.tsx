import { useState } from 'react'
import { api, errText } from '../api'
import { FUNCTIONS, isBitFunction, type ConnectionInfo, type FormatInfo, type Poll, type PollConfig } from '../types'
import { Modal } from './Modal'

export const blankPoll = (connectionId: string): PollConfig => ({
  id: '', name: '', connectionId, slaveId: 1, function: 3, address: 0, quantity: 10,
  scanRateMs: 1000, format: 'signed', plcAddressing: false, cellFormats: {}, tags: {},
})

interface Props {
  poll?: PollConfig
  connections: ConnectionInfo[]
  formats: FormatInfo[]
  onClose: () => void
  onSaved: (p: Poll) => void
}

export function PollDialog({ poll, connections, formats, onClose, onSaved }: Props) {
  const [form, setForm] = useState<PollConfig>(() => poll ?? blankPoll(connections[0]?.id ?? ''))
  // The address field is shown in the window's addressing base.
  const [addrText, setAddrText] = useState(String((poll?.address ?? 0) + (poll?.plcAddressing ? 1 : 0)))
  const [error, setError] = useState('')
  const set = <K extends keyof PollConfig>(k: K, v: PollConfig[K]) => setForm((f) => ({ ...f, [k]: v }))
  const bits = isBitFunction(form.function)
  const base = form.plcAddressing ? 1 : 0

  const save = async (start: boolean) => {
    setError('')
    const addr = Number(addrText) - base
    if (!Number.isInteger(addr) || addr < 0) return setError(`Address must be an integer ≥ ${base}`)
    try {
      const cfg = { ...form, address: addr }
      const saved = poll ? await api.updatePoll(cfg) : await api.createPoll(cfg)
      if (start && !saved.state.running) await api.startPoll(saved.config.id)
      onSaved(saved)
      onClose()
    } catch (e) {
      setError(errText(e))
    }
  }

  return (
    <Modal
      title={poll ? `Read/Write definition — ${poll.name}` : 'New poll window'}
      onClose={onClose}
      width={560}
      footer={
        <>
          <button className="btn outline" onClick={onClose}>Cancel</button>
          <button className="btn outline" onClick={() => save(false)}>{poll ? 'Apply' : 'Create'}</button>
          <button className="btn primary" onClick={() => save(true)}>{poll ? 'Apply & Start' : 'Create & Start'}</button>
        </>
      }
    >
      <div className="form">
        <label className="lbl">Window name</label>
        <input type="text" value={form.name} placeholder="(automatic)" onChange={(e) => set('name', e.target.value)} />
        <label className="lbl">Connection</label>
        <select value={form.connectionId} onChange={(e) => set('connectionId', e.target.value)}>
          {connections.length === 0 && <option value="">— create a connection first —</option>}
          {connections.map((c) => <option key={c.id} value={c.id}>{c.name} ({c.summary})</option>)}
        </select>
        <label className="lbl">Slave ID</label>
        <input type="number" min={0} max={255} value={form.slaveId} onChange={(e) => set('slaveId', Number(e.target.value))} />
        <label className="lbl">Function</label>
        <select value={form.function} onChange={(e) => set('function', Number(e.target.value))}>
          {FUNCTIONS.map((f) => <option key={f.code} value={f.code}>{f.label}</option>)}
        </select>
        <label className="lbl">Address</label>
        <input type="number" min={base} value={addrText} onChange={(e) => setAddrText(e.target.value)} />
        <label className="check full" style={{ gridColumn: 2 }}>
          <input
            type="checkbox"
            checked={form.plcAddressing}
            onChange={(e) => {
              const on = e.target.checked
              setAddrText(String(Math.max(0, Number(addrText) + (on ? 1 : -1))))
              set('plcAddressing', on)
            }}
          />
          PLC addresses (base 1)
        </label>
        <label className="lbl">Quantity</label>
        <input type="number" min={1} max={bits ? 2000 : 125} value={form.quantity} onChange={(e) => set('quantity', Number(e.target.value))} />
        <span className="hint">1–{bits ? 2000 : 125} {bits ? 'bits' : 'registers'}</span>
        <label className="lbl">Scan rate</label>
        <div className="inline">
          <input type="number" min={10} value={form.scanRateMs} onChange={(e) => set('scanRateMs', Number(e.target.value))} /> <span className="muted">ms</span>
        </div>
        {!bits && (
          <>
            <label className="lbl">Display format</label>
            <select value={form.format} onChange={(e) => set('format', e.target.value)}>
              {['16-bit', '32-bit', '64-bit'].map((g) => (
                <optgroup key={g} label={g}>
                  {formats.filter((f) => f.group === g).map((f) => <option key={f.id} value={f.id}>{f.label}</option>)}
                </optgroup>
              ))}
            </select>
            <span className="hint">Right-click a cell to override the format of a single value.</span>
          </>
        )}
        {error && <div className="alert full">{error}</div>}
      </div>
    </Modal>
  )
}
