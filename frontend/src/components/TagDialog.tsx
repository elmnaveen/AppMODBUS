import { useState } from 'react'
import { api, errText } from '../api'
import type { AlarmLimits, Cell, Poll, Tag } from '../types'
import { Modal } from './Modal'

export const blankTag = (): Tag => ({ name: '', gain: 1, offset: 0, unit: '', decimals: -1 })

interface Props {
  poll: Poll
  cell: Cell
  onClose: () => void
}

/** Edits the name, scaling and unit of one register. */
export function TagDialog({ poll, cell, onClose }: Props) {
  const existing = poll.config.tags?.[String(cell.address)]
  const [tag, setTag] = useState<Tag>(existing ? { ...existing } : blankTag())
  const [gainText, setGainText] = useState(String(existing?.gain ?? 1))
  const [offsetText, setOffsetText] = useState(String(existing?.offset ?? 0))
  const [error, setError] = useState('')
  const bits = poll.config.function <= 2
  const al = existing?.alarm
  const txt = (v: number | null | undefined) => (v === null || v === undefined ? '' : String(v))
  const [alarmOn, setAlarmOn] = useState(!!al?.enabled)
  const [lim, setLim] = useState({ hiHi: txt(al?.hiHi), hi: txt(al?.hi), lo: txt(al?.lo), loLo: txt(al?.loLo), deadband: txt(al?.deadband ?? 0) })
  const [onState, setOnState] = useState<number>(al?.onState ?? 1)
  const setL = (k: keyof typeof lim, v: string) => setLim((x) => ({ ...x, [k]: v }))
  const base = poll.config.plcAddressing ? 1 : 0
  const set = <K extends keyof Tag>(k: K, v: Tag[K]) => setTag((t) => ({ ...t, [k]: v }))

  const gain = Number(gainText)
  const offset = Number(offsetText)
  const rawSample = Number(cell.rawText ?? cell.value)
  const preview = !bits && Number.isFinite(rawSample) && Number.isFinite(gain) && Number.isFinite(offset)
    ? rawSample * gain + offset : null

  // Blank limits are off; returns an error text for invalid numbers.
  const buildAlarm = (): AlarmLimits | null | string => {
    if (bits) return alarmOn ? { enabled: true, hiHi: null, hi: null, lo: null, loLo: null, deadband: 0, onState } : null
    const num = (s: string, label: string) => {
      if (s.trim() === '') return null
      const n = Number(s)
      if (!Number.isFinite(n)) throw new Error(`${label} must be a number`)
      return n
    }
    try {
      const a: AlarmLimits = {
        enabled: alarmOn, hiHi: num(lim.hiHi, 'High-high'), hi: num(lim.hi, 'High'), lo: num(lim.lo, 'Low'), loLo: num(lim.loLo, 'Low-low'),
        deadband: num(lim.deadband, 'Deadband') ?? 0, onState: null,
      }
      if (!a.enabled && a.hiHi === null && a.hi === null && a.lo === null && a.loLo === null) return null
      if (a.enabled && a.hiHi === null && a.hi === null && a.lo === null && a.loLo === null) return 'Enter at least one limit, or turn the alarm off'
      return a
    } catch (e) {
      return (e as Error).message
    }
  }

  const save = async () => {
    const alarm = buildAlarm()
    if (typeof alarm === 'string') return setError(alarm)
    if (!bits && (!Number.isFinite(gain) || gain === 0)) return setError('Gain must be a non-zero number')
    if (!bits && !Number.isFinite(offset)) return setError('Offset must be a number')
    try {
      await api.setTag(poll.config.id, cell.address, { ...tag, gain: bits ? 1 : gain, offset: bits ? 0 : offset, alarm })
      onClose()
    } catch (e) {
      setError(errText(e))
    }
  }
  const remove = async () => {
    await api.setTag(poll.config.id, cell.address, null)
    onClose()
  }

  return (
    <Modal
      title={`Tag — address ${cell.address + base}`}
      onClose={onClose}
      width={480}
      footer={
        <>
          {existing && <button className="btn outline danger" onClick={remove}>Remove tag</button>}
          <span style={{ flex: 1 }} />
          <button className="btn outline" onClick={onClose}>Cancel</button>
          <button className="btn primary" onClick={save}>Save</button>
        </>
      }
    >
      <div className="form">
        <label className="lbl">Name</label>
        <input type="text" autoFocus value={tag.name} placeholder="e.g. Voltage L1" onChange={(e) => set('name', e.target.value)} />
        {!bits && (
          <>
            <label className="lbl">Gain (×)</label>
            <input type="text" className="mono" value={gainText} onChange={(e) => setGainText(e.target.value)} />
            <label className="lbl">Offset (+)</label>
            <input type="text" className="mono" value={offsetText} onChange={(e) => setOffsetText(e.target.value)} />
            <span className="hint">Shown value = raw × gain + offset. Writes convert back to raw.</span>
          </>
        )}
        <label className="lbl">Unit</label>
        <input type="text" value={tag.unit} placeholder="e.g. V, A, kWh, °C" onChange={(e) => set('unit', e.target.value)} />
        {!bits && (
          <>
            <label className="lbl">Decimals</label>
            <select value={tag.decimals} onChange={(e) => set('decimals', Number(e.target.value))}>
              <option value={-1}>Automatic (from gain)</option>
              {[0, 1, 2, 3, 4, 5, 6].map((d) => <option key={d} value={d}>{d}</option>)}
            </select>
          </>
        )}
        <label className="lbl">Note</label>
        <input type="text" value={tag.note ?? ''} onChange={(e) => set('note', e.target.value)} />
        <div className="full tag-section">Alarm</div>
        <label className="lbl">Alarm</label>
        <label className="check">
          <input type="checkbox" checked={alarmOn} onChange={(e) => setAlarmOn(e.target.checked)} />
          {bits ? 'Raise an alarm on this bit' : 'Raise alarms at these limits'}
        </label>
        {bits ? (
          <>
            <label className="lbl">Alarm when</label>
            <div className="seg">
              <button className={onState === 1 ? 'on' : ''} disabled={!alarmOn} onClick={() => setOnState(1)}>Bit is 1</button>
              <button className={onState === 0 ? 'on' : ''} disabled={!alarmOn} onClick={() => setOnState(0)}>Bit is 0</button>
            </div>
          </>
        ) : (
          <>
            <label className="lbl">High-high / High</label>
            <div className="inline">
              <input type="text" className="mono" disabled={!alarmOn} placeholder="off" value={lim.hiHi} onChange={(e) => setL('hiHi', e.target.value)} />
              <input type="text" className="mono" disabled={!alarmOn} placeholder="off" value={lim.hi} onChange={(e) => setL('hi', e.target.value)} />
            </div>
            <label className="lbl">Low / Low-low</label>
            <div className="inline">
              <input type="text" className="mono" disabled={!alarmOn} placeholder="off" value={lim.lo} onChange={(e) => setL('lo', e.target.value)} />
              <input type="text" className="mono" disabled={!alarmOn} placeholder="off" value={lim.loLo} onChange={(e) => setL('loLo', e.target.value)} />
            </div>
            <label className="lbl">Deadband</label>
            <input type="text" className="mono" disabled={!alarmOn} value={lim.deadband} onChange={(e) => setL('deadband', e.target.value)} />
            <span className="hint">In {tag.unit || 'engineering units'}, after scaling. An alarm clears only once the value is this far back inside the limit. High-high and low-low are critical; high and low are warnings.</span>
          </>
        )}
        {preview !== null && (
          <div className="full tag-preview mono">
            raw {rawSample} → <b>{Number(preview.toFixed(6))}</b> {tag.unit}
          </div>
        )}
        {error && <div className="alert full">{error}</div>}
      </div>
    </Modal>
  )
}
