import { memo, useRef, useState } from 'react'
import { api, errText } from '../api'
import { fc2, isBitFunction, isWritable, type Cell, type FormatInfo, type Poll } from '../types'
import { ContextMenu } from './ContextMenu'
import { IconGear, IconOnce, IconPlay, IconStop, IconX } from './Icons'

const ROWS = 10

interface Props {
  poll: Poll
  connectionName: string
  formats: FormatInfo[]
  focused: boolean
  onFocus: () => void
  onEdit: () => void
  onClose: () => void
  onWrite: (cell: Cell) => void
  onTag: (cell: Cell) => void
  onError: (msg: string) => void
}

function PollWindowImpl({ poll, connectionName, formats, focused, onFocus, onEdit, onClose, onWrite, onTag, onError }: Props) {
  const { config: c, state: s } = poll
  const [sel, setSel] = useState<number | null>(null)
  const [menu, setMenu] = useState<{ x: number; y: number; cell: Cell } | null>(null)
  const bits = isBitFunction(c.function)
  const base = c.plcAddressing ? 1 : 0

  // Remember the previous values so changed cells can flash.
  const prev = useRef<Map<number, string>>(new Map())
  const changed = new Set<number>()
  for (const cell of s.cells) {
    const p = prev.current.get(cell.address)
    if (p !== undefined && p !== cell.value && cell.span > 0) changed.add(cell.address)
  }
  prev.current = new Map(s.cells.map((x) => [x.address, x.value]))

  const run = (fn: () => Promise<void>) => fn().catch((e) => onError(errText(e)))
  const fmtLabel = (id: string) => formats.find((f) => f.id === id)?.label ?? id
  const wide = !bits && (c.format === 'binary' || Object.values(c.cellFormats).some((f) => f === 'binary' || f.startsWith('double') || f.startsWith('long64')))
  const valueWidth = wide ? 170 : bits ? 70 : 110
  // A name column appears once any register of the window has a name.
  const named = s.cells.some((x) => x.name)
  const hasUnits = s.cells.some((x) => x.unit)

  const columns: Cell[][] = []
  for (let i = 0; i < s.cells.length; i += ROWS) columns.push(s.cells.slice(i, i + ROWS))

  return (
    <div className={`pw${focused ? ' focused' : ''}`} onMouseDown={onFocus}>
      <div className="pw-head">
        <span className="pw-title">{c.name}</span>
        <span className={`pill${s.running ? ' run' : ''}`}>{s.running ? 'Polling' : 'Stopped'}</span>
        <span className="pw-stats">
          <span>Tx = <b>{s.tx}</b></span>
          <span>Err = <b style={s.errors ? { color: 'var(--err)' } : undefined}>{s.errors}</b></span>
          <span>ID = <b>{c.slaveId}</b></span>
          <span>F = <b>{fc2(c.function)}</b></span>
          <span>SR = <b>{c.scanRateMs}</b> ms</span>
        </span>
        <span className="pw-actions">
          {s.running ? (
            <button className="btn sm" onClick={() => run(() => api.stopPoll(c.id))} title="Stop polling"><IconStop />Stop</button>
          ) : (
            <button className="btn sm" onClick={() => run(() => api.startPoll(c.id))} title="Start polling"><IconPlay />Start</button>
          )}
          <button className="btn sm icon" disabled={s.running} onClick={() => run(() => api.readOnce(c.id))} title="Read once"><IconOnce /></button>
          <button className="btn sm icon" onClick={onEdit} title="Read/write definition"><IconGear /></button>
          <button className="btn sm icon" onClick={onClose} title="Close window"><IconX /></button>
        </span>
      </div>
      <div className={`pw-status${s.lastError ? ' error' : ''}`}>
        {s.lastError ? (
          <span>{s.lastError}</span>
        ) : s.hasData ? (
          <span>
            {connectionName} · {c.quantity} {bits ? 'bits' : 'registers'} from {c.address + base} · last response {s.responseMs} ms ·{' '}
            {new Date(s.updatedAt).toLocaleTimeString()}
          </span>
        ) : (
          <span>{connectionName || 'No connection'} · press Start to poll</span>
        )}
      </div>
      <div className="grid-wrap">
        <div className={`grid${named ? ' named' : ''}`} style={{ ['--vw' as string]: `${valueWidth + (hasUnits ? 34 : 0)}px` }}>
          {columns.map((col, ci) => (
            <div className="gcol" key={ci}>
              <div className="gcol-head"><span>Addr</span>{named && <span>Name</span>}<span style={{ textAlign: 'right' }}>{bits ? 'Bit' : 'Value'}</span></div>
              {col.map((cell) => {
                const override = c.cellFormats[String(cell.address)] !== undefined
                return (
                  <div
                    key={cell.address}
                    className={`gcell${bits ? ' bit' : ''}${sel === cell.address ? ' sel' : ''}`}
                    onClick={() => setSel(cell.address)}
                    onDoubleClick={() => {
                      if (!isWritable(c.function)) return onError(`${bits ? 'Discrete inputs' : 'Input registers'} are read-only`)
                      onWrite(cell.span === 0 ? s.cells.find((x) => x.span > 0 && x.address < cell.address && x.address + x.span > cell.address) ?? cell : cell)
                    }}
                    onContextMenu={(e) => {
                      e.preventDefault()
                      setSel(cell.address)
                      setMenu({ x: e.clientX, y: e.clientY, cell: cell.span === 0 ? s.cells.find((x) => x.span > 0 && x.address < cell.address && x.address + x.span > cell.address) ?? cell : cell })
                    }}
                    title={cell.span > 0 && !bits
                      ? `${cell.name ? cell.name + ' · ' : ''}${fmtLabel(cell.format)}${cell.rawText ? ` · value ${cell.rawText}` : ''} · raw 0x${cell.raw.toString(16).toUpperCase().padStart(4, '0')}`
                      : cell.name || undefined}
                  >
                    <span className="a">{cell.address + base}</span>
                    {named && <span className="n">{cell.span === 0 ? '' : cell.name}</span>}
                    {cell.span === 0 ? (
                      <span className="v cont">⋯</span>
                    ) : bits ? (
                      <span key={cell.value} className={`v${changed.has(cell.address) ? ' chg' : ''}`}>
                        {cell.value}
                        <span className={`bitdot${cell.value === '1' ? ' on' : ''}`} />
                      </span>
                    ) : (
                      <span key={cell.value} className={`v${override ? ' override' : ''}${cell.rawText ? ' scaled' : ''}${changed.has(cell.address) ? ' chg' : ''}`}>
                        {cell.value}{cell.unit && <small className="u">{cell.unit}</small>}
                      </span>
                    )}
                  </div>
                )
              })}
            </div>
          ))}
        </div>
      </div>
      {menu && (
        <ContextMenu x={menu.x} y={menu.y} onClose={() => setMenu(null)}>
          <button onClick={() => { setMenu(null); onTag(menu.cell) }}>{menu.cell.name ? `Edit tag “${menu.cell.name}”…` : 'Name / scale this register…'}</button>
          {isWritable(c.function) && (
            <button onClick={() => { setMenu(null); onWrite(menu.cell) }}>Write value…</button>
          )}
          {!bits && <hr />}
          {!bits && ['16-bit', '32-bit', '64-bit'].map((g) => (
            <div key={g}>
              <div className="grp">{g}</div>
              {formats.filter((f) => f.group === g).map((f) => (
                <button
                  key={f.id}
                  className={menu.cell.format === f.id ? 'cur' : ''}
                  onClick={() => {
                    setMenu(null)
                    run(() => api.setCellFormat(c.id, menu.cell.address, f.id))
                  }}
                >
                  {f.label}
                </button>
              ))}
            </div>
          ))}
          {!bits && <hr />}
          {!bits && (
            <button onClick={() => { setMenu(null); run(() => api.setCellFormat(c.id, menu.cell.address, '')) }}>
              Reset to window format ({fmtLabel(c.format)})
            </button>
          )}
        </ContextMenu>
      )}
    </div>
  )
}

export const PollWindow = memo(PollWindowImpl)
