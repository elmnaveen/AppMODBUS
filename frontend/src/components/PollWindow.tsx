import { memo, useRef, useState } from 'react'
import { api, errText } from '../api'
import { Chart } from '../charts/Chart'
import { addSeries, defaultInline, MAX_SERIES, resolveSeries, type ChartSeries, type InlineChart } from '../charts/model'
import { ALARM_LEVEL_LABELS, fc2, isBitFunction, isWritable, type AlarmLevel, type Cell, type ConnectionInfo, type FormatInfo, type Poll } from '../types'
import { ContextMenu, Dropdown, type MenuEntry } from './Menu'
import { formatGroups } from './formatMenu'
import { IconChart, IconGear, IconOnce, IconPen, IconPlay, IconStop, IconTag, IconX } from './Icons'

const ROWS = 10

const alarmClass = (l: AlarmLevel) => (l === 'hihi' || l === 'lolo' || l === 'comm' ? 'crit' : 'warn')
/** Arrow for the limit side plus a label in the tooltip, so state is not colour alone. */
function AlarmMark({ level }: { level: AlarmLevel }) {
  const glyph = level === 'hihi' ? '⏶⏶' : level === 'hi' ? '⏶' : level === 'lolo' ? '⏷⏷' : level === 'lo' ? '⏷' : '!'
  return <span className="al-mark" title={`${ALARM_LEVEL_LABELS[level]} alarm`}>{glyph}</span>
}

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
  onInfo: (msg: string) => void
  // Charts
  theme: 'light' | 'dark'
  connections: ConnectionInfo[]
  inline?: InlineChart
  chartTargets: { id: string; name: string; full: boolean }[]
  onInline: (pollId: string, c: InlineChart) => void
  onChartCell: (pollId: string, address: number, target: string) => void
}

function PollWindowImpl({
  poll, connectionName, formats, focused, onFocus, onEdit, onClose, onWrite, onTag, onError, onInfo,
  theme, connections, inline, chartTargets, onInline, onChartCell,
}: Props) {
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

  const chartOpen = !!inline?.open
  const toggleChart = () => onInline(c.id, inline ? { ...inline, open: !inline.open } : defaultInline(poll))
  const unitOf = (x: ChartSeries) => s.cells.find((cell) => cell.address === x.address)?.unit ?? ''
  const inInline = (address: number) => !!inline?.series.some((x) => x.address === address)
  const toggleInline = (cell: Cell) => {
    const cur = inline ?? { open: true, rangeSec: 300, series: [] }
    const series = inInline(cell.address)
      ? cur.series.filter((x) => x.address !== cell.address)
      : addSeries(cur.series, c.id, cell.address, unitOf, cell.unit ?? '')
    onInline(c.id, { ...cur, open: true, series })
  }

  // Header format picker: changes the whole window at once while it keeps polling.
  const windowFormatMenu = (): MenuEntry[] => {
    const setFormat = (id: string) => run(async () => { await api.updatePoll({ ...c, format: id }) })
    const groups = formatGroups(formats, c.format, setFormat)
    const entries: MenuEntry[] = groups.flatMap((g, i): MenuEntry[] => [...(i ? [{ type: 'sep' } as MenuEntry] : []), ...g.items])
    const overrides = Object.keys(c.cellFormats).length
    if (overrides) {
      entries.push({ type: 'sep' }, {
        label: `Clear ${overrides} register override${overrides > 1 ? 's' : ''}`,
        title: 'Registers given their own format (right-click → Format) go back to the window format',
        onSelect: () => run(async () => { await api.updatePoll({ ...c, cellFormats: {} }) }),
      })
    }
    return entries
  }

  // Right-click menu for one register: tag and write, a Chart submenu, and a
  // Format submenu grouped by width, then by type (e.g. 32-bit ▸ Float ▸ CD AB).
  const cellMenu = (cell: Cell): MenuEntry[] => {
    const shown = inInline(cell.address)
    const entries: MenuEntry[] = [
      { label: cell.name ? `Edit tag “${cell.name}”…` : 'Name / scale this register…', icon: <IconTag />, onSelect: () => onTag(cell) },
    ]
    if (isWritable(c.function)) entries.push({ label: 'Write value…', icon: <IconPen />, onSelect: () => onWrite(cell) })
    entries.push({ type: 'sep' }, {
      type: 'sub', label: 'Chart', icon: <IconChart />, items: [
        { label: 'Show in window chart', checked: shown, disabled: !shown && (inline?.series.length ?? 0) >= MAX_SERIES, onSelect: () => toggleInline(cell) },
        { type: 'sep' },
        ...chartTargets.map((t): MenuEntry => ({ label: `Add to “${t.name}”`, disabled: t.full, onSelect: () => onChartCell(c.id, cell.address, t.id) })),
        { label: 'Add to new chart', onSelect: () => onChartCell(c.id, cell.address, 'new') },
      ],
    })
    if (bits) return entries
    const cur = cell.format || c.format
    const pick = (id: string) => () => run(() => api.setCellFormat(c.id, cell.address, id))
    const groups = formatGroups(formats, cur, (id) => pick(id)()).map((g): MenuEntry => (
      { type: 'sub', label: g.group, icon: g.here ? <span className="wm-dot" /> : undefined, items: g.items }
    ))
    entries.push({ type: 'sep' }, {
      type: 'sub', label: 'Format', items: [
        ...groups,
        { type: 'sep' },
        { label: `Use window format (${fmtLabel(c.format)})`, radio: true, checked: !cell.format, onSelect: pick('') },
      ],
    })
    return entries
  }

  const columns: Cell[][] = []
  for (let i = 0; i < s.cells.length; i += ROWS) columns.push(s.cells.slice(i, i + ROWS))

  return (
    <div className={`pw${focused ? ' focused' : ''}${chartOpen ? ' with-chart' : ''}`} onMouseDown={onFocus}>
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
          {!bits && (
            <Dropdown
              className="sm fmt-pick"
              label={fmtLabel(c.format)}
              title="Display format for this window (applies immediately)"
              entries={windowFormatMenu()}
            />
          )}
          <button
            className={`btn sm rec${c.record ? ' on' : ''}`}
            title={c.record ? 'Recording to disk — click to stop' : 'Record values to disk'}
            onClick={() => run(() => api.setPollRecording(c.id, !c.record))}
          >
            <span className="rec-dot" />REC
          </button>
          <button className={`btn sm${chartOpen ? ' on' : ''}`} onClick={toggleChart} title={chartOpen ? 'Hide the chart' : 'Chart this window'}>
            <IconChart />Chart
          </button>
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
                      <span key={cell.value} className={`v${changed.has(cell.address) ? ' chg' : ''}${cell.alarm ? ` al ${alarmClass(cell.alarm)}` : ''}`}>
                        {cell.alarm && <AlarmMark level={cell.alarm} />}
                        {cell.value}
                        <span className={`bitdot${cell.value === '1' ? ' on' : ''}`} />
                      </span>
                    ) : (
                      <span key={cell.value} className={`v${override ? ' override' : ''}${cell.rawText ? ' scaled' : ''}${changed.has(cell.address) ? ' chg' : ''}${cell.alarm ? ` al ${alarmClass(cell.alarm)}` : ''}`}>
                        {cell.alarm && <AlarmMark level={cell.alarm} />}
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
      {chartOpen && inline && (
        <div className="pw-chart">
          <Chart
            compact
            series={resolveSeries(inline.series, { [c.id]: poll }, connections, false)}
            rangeSec={inline.rangeSec}
            onRange={(rangeSec) => onInline(c.id, { ...inline, rangeSec })}
            theme={theme}
            title={c.name}
            onAxis={(i) => onInline(c.id, { ...inline, series: inline.series.map((x, j) => (j === i ? { ...x, axis: x.axis === 'L' ? 'R' : 'L' } : x)) })}
            onRemove={(i) => onInline(c.id, { ...inline, series: inline.series.filter((_, j) => j !== i) })}
            onError={onError}
            onInfo={onInfo}
            empty="Right-click a value and choose “Show in window chart”."
          />
        </div>
      )}
      {menu && <ContextMenu x={menu.x} y={menu.y} entries={cellMenu(menu.cell)} onClose={() => setMenu(null)} />}
    </div>
  )
}

export const PollWindow = memo(PollWindowImpl)
