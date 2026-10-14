import { useState } from 'react'
import { api, errText } from '../api'
import type { Cell, ConnectionInfo, FormatInfo, Poll } from '../types'
import { Modal } from './Modal'

// ---------- write from a poll window cell (double-click) ----------

interface CellProps {
  poll: Poll
  cell: Cell
  formats: FormatInfo[]
  onClose: () => void
}

export function WriteCellDialog({ poll, cell, formats, onClose }: CellProps) {
  const c = poll.config
  const coil = c.function === 1
  const fmt = formats.find((f) => f.id === cell.format)
  const multiReg = (fmt?.registers ?? 1) > 1
  const [value, setValue] = useState(cell.value === '-' ? '' : cell.value)
  const [force, setForce] = useState(false)
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)
  const base = c.plcAddressing ? 1 : 0

  const title = coil
    ? force ? 'Write Multiple Coils (FC15)' : 'Write Single Coil (FC05)'
    : multiReg || force ? 'Write Multiple Registers (FC16)' : 'Write Single Register (FC06)'

  const send = async (v = value) => {
    setBusy(true)
    setError('')
    try {
      await api.writeCell(c.id, cell.address, v, force)
      onClose()
    } catch (e) {
      setError(errText(e))
    } finally {
      setBusy(false)
    }
  }

  return (
    <Modal
      title={title}
      onClose={onClose}
      width={440}
      footer={
        <>
          <button className="btn outline" onClick={onClose}>Cancel</button>
          {!coil && <button className="btn primary" disabled={busy} onClick={() => send()}>Send</button>}
        </>
      }
    >
      <div className="form">
        <label className="lbl">Slave ID</label>
        <span className="mono">{c.slaveId}</span>
        <label className="lbl">Address</label>
        <span className="mono">{cell.address + base}{multiReg ? ` – ${cell.address + base + (fmt!.registers - 1)}` : ''}</span>
        {coil ? (
          <>
            <label className="lbl">Value</label>
            <div className="inline">
              <button className="btn outline" disabled={busy} onClick={() => send('1')}><span className="bitdot on" /> On</button>
              <button className="btn outline" disabled={busy} onClick={() => send('0')}><span className="bitdot" /> Off</button>
            </div>
          </>
        ) : (
          <>
            <label className="lbl">Format</label>
            <span>{fmt?.label ?? cell.format}</span>
            <label className="lbl">Value</label>
            <input
              type="text"
              className="mono"
              autoFocus
              value={value}
              onChange={(e) => setValue(e.target.value)}
              onKeyDown={(e) => e.key === 'Enter' && send()}
              onFocus={(e) => e.target.select()}
            />
          </>
        )}
        {!(multiReg && !coil) && (
          <label className="check full" style={{ gridColumn: 2 }}>
            <input type="checkbox" checked={force} onChange={(e) => setForce(e.target.checked)} />
            Use function {coil ? '15 (Write Multiple Coils)' : '16 (Write Multiple Registers)'}
          </label>
        )}
        {error && <div className="alert full">{error}</div>}
      </div>
    </Modal>
  )
}

// ---------- manual write (toolbar) ----------

const WRITE_FUNCS = [
  { code: 5, label: '05 Write Single Coil' },
  { code: 6, label: '06 Write Single Register' },
  { code: 15, label: '15 Write Multiple Coils' },
  { code: 16, label: '16 Write Multiple Registers' },
]

interface ManualProps {
  connections: ConnectionInfo[]
  formats: FormatInfo[]
  defaults?: { connectionId: string; slaveId: number }
  onClose: () => void
}

export function ManualWriteDialog({ connections, formats, defaults, onClose }: ManualProps) {
  const [connId, setConnId] = useState(defaults?.connectionId ?? connections[0]?.id ?? '')
  const [slave, setSlave] = useState(defaults?.slaveId ?? 1)
  const [fn, setFn] = useState(6)
  const [address, setAddress] = useState(0)
  const [format, setFormat] = useState('signed')
  const [text, setText] = useState('')
  const [coilOn, setCoilOn] = useState(true)
  const [error, setError] = useState('')
  const [ok, setOk] = useState('')

  const regFormats = fn === 6 ? formats.filter((f) => f.registers === 1) : formats

  const send = async () => {
    setError('')
    setOk('')
    try {
      let values: number[] = []
      if (fn === 5) values = [coilOn ? 1 : 0]
      else {
        const items = text.split(/[,;\s]+/).filter(Boolean)
        if (items.length === 0) throw new Error('Enter at least one value')
        if (fn === 15) {
          values = items.map((v) => {
            if (v !== '0' && v !== '1') throw new Error(`"${v}" is not 0 or 1`)
            return Number(v)
          })
        } else {
          for (const v of items) values.push(...(await api.parseValue(format, v)))
          if (fn === 6 && values.length !== 1) throw new Error('Function 06 writes exactly one register')
        }
      }
      await api.write({ connectionId: connId, slaveId: slave, function: fn, address, values })
      setOk(`Wrote ${values.length} ${fn === 5 || fn === 15 ? 'coil(s)' : 'register(s)'} at address ${address}`)
    } catch (e) {
      setError(errText(e))
    }
  }

  return (
    <Modal
      title="Write"
      onClose={onClose}
      width={520}
      footer={
        <>
          <button className="btn outline" onClick={onClose}>Close</button>
          <button className="btn primary" disabled={!connId} onClick={send}>Send</button>
        </>
      }
    >
      <div className="form">
        <label className="lbl">Connection</label>
        <select value={connId} onChange={(e) => setConnId(e.target.value)}>
          {connections.map((c) => <option key={c.id} value={c.id}>{c.name} ({c.summary})</option>)}
        </select>
        <label className="lbl">Function</label>
        <select value={fn} onChange={(e) => setFn(Number(e.target.value))}>
          {WRITE_FUNCS.map((f) => <option key={f.code} value={f.code}>{f.label}</option>)}
        </select>
        <label className="lbl">Slave ID</label>
        <input type="number" min={0} max={255} value={slave} onChange={(e) => setSlave(Number(e.target.value))} />
        <span className="hint">0 = broadcast (no response expected)</span>
        <label className="lbl">Address</label>
        <input type="number" min={0} max={65535} value={address} onChange={(e) => setAddress(Number(e.target.value))} />
        <span className="hint">Protocol address (base 0)</span>
        {fn === 5 ? (
          <>
            <label className="lbl">Value</label>
            <div className="seg">
              <button className={coilOn ? 'on' : ''} onClick={() => setCoilOn(true)}>On (FF00)</button>
              <button className={!coilOn ? 'on' : ''} onClick={() => setCoilOn(false)}>Off (0000)</button>
            </div>
          </>
        ) : (
          <>
            {fn !== 15 && (
              <>
                <label className="lbl">Value format</label>
                <select value={regFormats.some((f) => f.id === format) ? format : 'signed'} onChange={(e) => setFormat(e.target.value)}>
                  {regFormats.map((f) => <option key={f.id} value={f.id}>{f.label}</option>)}
                </select>
              </>
            )}
            <label className="lbl">{fn === 6 ? 'Value' : 'Values'}</label>
            <textarea rows={fn === 6 ? 1 : 3} value={text} onChange={(e) => setText(e.target.value)} placeholder={fn === 15 ? '1, 0, 1, 1' : fn === 16 ? '100, 200, 300' : '1234'} />
            {fn !== 6 && <span className="hint">Separate values with commas or spaces.</span>}
          </>
        )}
        {error && <div className="alert full">{error}</div>}
        {ok && <div className="full" style={{ color: 'var(--ok)' }}>✓ {ok}</div>}
      </div>
    </Modal>
  )
}
