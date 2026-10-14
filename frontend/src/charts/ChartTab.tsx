import { memo, useState } from 'react'
import { Modal } from '../components/Modal'
import { IconX } from '../components/Icons'
import { isBitFunction, type ConnectionInfo, type Poll } from '../types'
import { Chart } from './Chart'
import { addSeries, chartableCells, MAX_SERIES, resolveSeries, type ChartDef, type ChartSeries } from './model'

interface TabProps {
  chart: ChartDef
  polls: Record<string, Poll>
  connections: ConnectionInfo[]
  theme: 'light' | 'dark'
  focused: boolean
  onFocus: () => void
  onChange: (c: ChartDef) => void
  onAdd: () => void
  onClose: () => void
  onError: (m: string) => void
  onInfo: (m: string) => void
}

/** A chart tab combining registers from any poll windows. */
function ChartTabImpl({ chart, polls, connections, theme, focused, onFocus, onChange, onAdd, onClose, onError, onInfo }: TabProps) {
  const [name, setName] = useState<string | null>(null)
  const plot = resolveSeries(chart.series, polls, connections, true)
  const update = (series: ChartSeries[]) => onChange({ ...chart, series })
  return (
    <div className={`pw chart-pw${focused ? ' focused' : ''}`} onMouseDown={onFocus}>
      <div className="pw-head">
        {name === null ? (
          <span className="pw-title" title="Double-click to rename" onDoubleClick={() => setName(chart.name)}>{chart.name}</span>
        ) : (
          <input
            className="title-input"
            autoFocus
            value={name}
            onChange={(e) => setName(e.target.value)}
            onBlur={() => { if (name.trim()) onChange({ ...chart, name: name.trim() }); setName(null) }}
            onKeyDown={(e) => { if (e.key === 'Enter') e.currentTarget.blur(); if (e.key === 'Escape') setName(null) }}
          />
        )}
        <span className="pw-stats"><span>{chart.series.length}/{MAX_SERIES} series</span></span>
        <span className="pw-actions">
          <button className="btn sm icon" onClick={onClose} title="Close chart"><IconX /></button>
        </span>
      </div>
      <Chart
        series={plot}
        rangeSec={chart.rangeSec}
        onRange={(rangeSec) => onChange({ ...chart, rangeSec })}
        theme={theme}
        title={chart.name}
        onAxis={(i) => update(chart.series.map((s, j) => (j === i ? { ...s, axis: s.axis === 'L' ? 'R' : 'L' } : s)))}
        onRemove={(i) => update(chart.series.filter((_, j) => j !== i))}
        onAdd={chart.series.length < MAX_SERIES ? onAdd : undefined}
        onError={onError}
        onInfo={onInfo}
        empty={
          <>
            <p>Add registers from any poll window: use <b>Add series</b>, or right-click a value and choose <b>Add to “{chart.name}”</b>.</p>
            <button className="btn primary" onClick={onAdd}>Add series</button>
          </>
        }
      />
    </div>
  )
}

export const ChartTab = memo(ChartTabImpl)

interface AddProps {
  chart: ChartDef
  polls: Record<string, Poll>
  order: string[]
  initialPollId?: string
  onSave: (series: ChartSeries[]) => void
  onClose: () => void
}

/** Picks registers of a poll window to plot. */
export function AddSeriesDialog({ chart, polls, order, initialPollId, onSave, onClose }: AddProps) {
  const ids = order.filter((id) => polls[id])
  const [pollId, setPollId] = useState(initialPollId && polls[initialPollId] ? initialPollId : ids[0] ?? '')
  const [picked, setPicked] = useState<Set<string>>(new Set())
  const p = polls[pollId]
  const key = (pid: string, a: number) => `${pid}:${a}`
  const has = (pid: string, a: number) => chart.series.some((s) => s.pollId === pid && s.address === a)
  const room = MAX_SERIES - chart.series.length - picked.size
  const base = p?.config.plcAddressing ? 1 : 0

  const unitOf = (s: ChartSeries) => polls[s.pollId]?.state.cells.find((c) => c.address === s.address)?.unit ?? ''
  const save = () => {
    let series = chart.series
    for (const k of picked) {
      const [pid, a] = [k.slice(0, k.lastIndexOf(':')), Number(k.slice(k.lastIndexOf(':') + 1))]
      const unit = polls[pid]?.state.cells.find((c) => c.address === a)?.unit ?? ''
      series = addSeries(series, pid, a, unitOf, unit)
    }
    onSave(series)
    onClose()
  }

  return (
    <Modal
      title={`Add series to “${chart.name}”`}
      onClose={onClose}
      width={520}
      footer={
        <>
          <span className="muted">{chart.series.length + picked.size}/{MAX_SERIES} series</span>
          <span style={{ flex: 1 }} />
          <button className="btn outline" onClick={onClose}>Cancel</button>
          <button className="btn primary" disabled={picked.size === 0} onClick={save}>Add {picked.size || ''}</button>
        </>
      }
    >
      {ids.length === 0 ? (
        <p className="muted">Open a poll window first; charts plot the registers it reads.</p>
      ) : (
        <div className="form">
          <label className="lbl">Poll window</label>
          <select value={pollId} onChange={(e) => setPollId(e.target.value)}>
            {ids.map((id) => <option key={id} value={id}>{polls[id].config.name}</option>)}
          </select>
          {p && (
            <div className="pick-list full">
              {chartableCells(p).map((c) => {
                const k = key(p.config.id, c.address)
                const already = has(p.config.id, c.address)
                const on = already || picked.has(k)
                return (
                  <label key={k} className={`pick${already ? ' dim' : ''}`}>
                    <input
                      type="checkbox"
                      checked={on}
                      disabled={already || (!picked.has(k) && room <= 0)}
                      onChange={() => setPicked((cur) => { const n = new Set(cur); n.has(k) ? n.delete(k) : n.add(k); return n })}
                    />
                    <span className="mono a">{c.address + base}</span>
                    <span className="n">{c.name || (isBitFunction(p.config.function) ? 'Bit' : 'Register')}</span>
                    <span className="mono v">{c.value}{c.unit ? ` ${c.unit}` : ''}</span>
                  </label>
                )
              })}
              {chartableCells(p).length === 0 && <p className="muted">This window has no values yet.</p>}
            </div>
          )}
        </div>
      )}
    </Modal>
  )
}
