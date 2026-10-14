import { useState } from 'react'
import { api, errText } from '../api'
import type { Cell, Poll, Tag } from '../types'
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
  const base = poll.config.plcAddressing ? 1 : 0
  const set = <K extends keyof Tag>(k: K, v: Tag[K]) => setTag((t) => ({ ...t, [k]: v }))

  const gain = Number(gainText)
  const offset = Number(offsetText)
  const rawSample = Number(cell.rawText ?? cell.value)
  const preview = !bits && Number.isFinite(rawSample) && Number.isFinite(gain) && Number.isFinite(offset)
    ? rawSample * gain + offset : null

  const save = async () => {
    if (!bits && (!Number.isFinite(gain) || gain === 0)) return setError('Gain must be a non-zero number')
    if (!bits && !Number.isFinite(offset)) return setError('Offset must be a number')
    try {
      await api.setTag(poll.config.id, cell.address, { ...tag, gain: bits ? 1 : gain, offset: bits ? 0 : offset })
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
