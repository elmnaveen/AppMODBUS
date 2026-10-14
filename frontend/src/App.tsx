import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { api, errText, events } from './api'
import { ConnectionsDialog, blankConnection } from './components/ConnectionsDialog'
import {
  IconBell, IconBolt, IconBook, IconChart, IconFlask, IconPulse, IconDatabase, IconInfo, IconList, IconMoon, IconPen, IconPlay, IconPlug, IconPlus, IconStop, IconSun, IconTabs, IconX, IconWrench, IconEye, IconGear, IconFolder, IconSave, IconUnplug, IconHelp, IconLog,
} from './components/Icons'
import { PollDialog } from './components/PollDialog'
import { PollWindow } from './components/PollWindow'
import { SimulatorPanel, defaultSim } from './components/SimulatorPanel'
import { TrafficPanel } from './components/TrafficPanel'
import { AboutDialog, UpdateBanner, useUpdates } from './components/Updates'
import { DataDialog } from './components/DataDialog'
import { DiagnosticsDialog } from './components/Diagnostics'
import { AlarmsPanel, beep, loadAlarmSound } from './components/AlarmsPanel'
import { TestsDialog, parseSequences } from './components/Tests'
import { ProfilesDialog } from './components/Profiles'
import { TagDialog } from './components/TagDialog'
import { FileMenu, PortRemapDialog, UnsavedDialog, fileMenuEntries } from './components/Workspace'
import { MenuBar, type BarMenu } from './components/MenuBar'
import { PreferencesDialog } from './components/Preferences'
import { HelpDialog } from './components/HelpDialog'
import { AppLogDialog } from './components/AppLogDialog'
import { Logo } from './components/Logo'
import { Quit } from '../wailsjs/runtime/runtime'
import { Dropdown, type MenuEntry } from './components/Menu'
import { formatGroups } from './components/formatMenu'
import { ManualWriteDialog, WriteCellDialog } from './components/WriteDialogs'
import { useMenuStyle, useTheme } from './theme'
import { AddSeriesDialog, ChartTab } from './charts/ChartTab'
import { live } from './charts/live'
import {
  addSeries, emptyCharts, MAX_SERIES, newChartId, parseCharts, pruneCharts,
  type ChartDef, type ChartSeries, type ChartsDoc, type InlineChart,
} from './charts/model'
import { fc2, isBitFunction, type Alarm, type PollConfig, type AppInfo, type Sequence, type Cell, type LoadResult, type WorkspaceState, type ConnectionInfo, type FormatInfo, type Poll, type SimStatus } from './types'

type Dialog =
  | { kind: 'connections'; id?: string }
  | { kind: 'poll'; id?: string }
  | { kind: 'writeCell'; pollId: string; cell: Cell }
  | { kind: 'write'; fn?: number }
  | { kind: 'about' }
  | { kind: 'prefs' }
  | { kind: 'help' }
  | { kind: 'applog' }
  | { kind: 'tag'; pollId: string; cell: Cell }
  | { kind: 'profiles' }
  | { kind: 'data' }
  | { kind: 'addSeries'; chartId: string }
  | { kind: 'diag'; section?: 'stats' | 'ids' | 'map' | 'network' | 'device' }
  | { kind: 'tests' }
  | null

const isMac = navigator.platform.toUpperCase().includes('MAC')

export default function App() {
  const [theme, , setTheme] = useTheme()
  const [menuStyle, setMenuStyle] = useMenuStyle()
  const [mpToolbar, setMpToolbar] = useState(() => { try { return localStorage.getItem('appmodbus.mpToolbar') !== 'off' } catch { return true } })
  useEffect(() => { try { localStorage.setItem('appmodbus.mpToolbar', mpToolbar ? 'on' : 'off') } catch { /* ignore */ } }, [mpToolbar])
  const [polls, setPolls] = useState<Record<string, Poll>>({})
  const [order, setOrder] = useState<string[]>([])
  const [connections, setConnections] = useState<ConnectionInfo[]>([])
  const [formats, setFormats] = useState<FormatInfo[]>([])
  const [active, setActive] = useState<string>('')
  const [view, setView] = useState<'tabs' | 'tile'>('tabs')
  const [dialog, setDialog] = useState<Dialog>(null)
  const [showTraffic, setShowTraffic] = useState(true)
  const [showSim, setShowSim] = useState(false)
  const [sim, setSim] = useState<SimStatus | null>(null)
  const [info, setInfo] = useState<AppInfo | null>(null)
  const updates = useUpdates()
  const [toast, setToast] = useState<{ msg: string; error: boolean } | null>(null)
  const [logUnseen, setLogUnseen] = useState({ warn: 0, error: 0 })
  const [ws, setWs] = useState<WorkspaceState>({ path: '', name: 'Untitled', dirty: false, recent: [] })
  const [restored, setRestored] = useState(false)
  const [pending, setPending] = useState<null | (() => Promise<void>)>(null)
  const [remap, setRemap] = useState<LoadResult | null>(null)
  const [alarms, setAlarms] = useState<Alarm[]>([])
  const [showAlarms, setShowAlarms] = useState(false)
  const [alarmSound, setAlarmSound] = useState(loadAlarmSound)
  const seenAlarms = useRef(new Set<string>())
  const [charts, setCharts] = useState<ChartsDoc>(emptyCharts)
  const sentCharts = useRef(JSON.stringify(emptyCharts()))
  const [sequences, setSequences] = useState<Sequence[]>([])
  const sentSeqs = useRef('[]')

  const notify = useCallback((msg: string, error = true) => {
    setToast({ msg, error })
    if (error) api.logUI('error', msg).catch(() => { /* logging must never break the UI */ })
    setTimeout(() => setToast((t) => (t?.msg === msg ? null : t)), 4000)
  }, [])

  useEffect(() => {
    api.appInfo().then(setInfo)
    api.formats().then(setFormats)
    api.restoreSession().then(applyLoad).finally(() => setRestored(true))
    api.alarms().then(setAlarms)
    const offs = [
      events.onPoll((p) => {
        live.ingest(p)
        setPolls((cur) => ({ ...cur, [p.config.id]: p }))
      }),
      events.onConnections(setConnections),
      // An operator step needs an answer: bring the test window back.
      events.onSequence((r) => { if (r.prompt) setDialog((d) => (d?.kind === 'tests' ? d : { kind: 'tests' })) }),
      events.onAlarms((list) => {
        setAlarms(list)
        // Open the panel when a new alarm is raised (not on every update).
        const fresh = list.some((a) => a.active && !a.acked && !seenAlarms.current.has(`${a.id}@${a.since}`))
        list.forEach((a) => seenAlarms.current.add(`${a.id}@${a.since}`))
        if (fresh) setShowAlarms(true)
      }),
      events.onWorkspace(setWs),
    ]
    return () => offs.forEach((off) => off())
  }, [])

  // Rebuilds the UI from a freshly loaded workspace.
  const applyLoad = async (r: LoadResult) => {
    const [ps, cs, st] = await Promise.all([api.polls(), api.connections(), api.simStatus()])
    setPolls(Object.fromEntries(ps.map((p) => [p.config.id, p])))
    setOrder(ps.map((p) => p.config.id))
    setConnections(cs)
    setSim(st)
    setWs(r.state)
    setView(r.layout.view === 'tile' ? 'tile' : 'tabs')
    setShowTraffic(r.layout.showTraffic)
    setShowSim(r.layout.showSimulator)
    const doc = pruneCharts(parseCharts(r.charts), new Set(ps.map((p) => p.config.id)))
    sentCharts.current = JSON.stringify(doc)
    setCharts(doc)
    const seqs = parseSequences(r.sequences)
    sentSeqs.current = JSON.stringify(seqs)
    setSequences(seqs)
    const ids = [...ps.map((p) => p.config.id), ...doc.tabs.map((t) => t.id)]
    setActive(ids.includes(r.layout.activeId) ? r.layout.activeId : ids[0] ?? '')
    if (r.warnings.length) notify(`Some items could not be loaded: ${r.warnings.join('; ')}`)
    if (r.missingPorts.length) setRemap(r)
  }

  // The layout is saved with the workspace.
  useEffect(() => {
    if (restored) api.setLayout({ view, activeId: active, showTraffic, showSimulator: showSim })
  }, [restored, view, active, showTraffic, showSim])

  useEffect(() => {
    const json = JSON.stringify(sequences)
    if (restored && json !== sentSeqs.current) {
      sentSeqs.current = json
      api.setSequences(sequences)
    }
  }, [restored, sequences])

  // Beep while alarms are unacknowledged.
  const unacked = alarms.filter((a) => !a.acked)
  const unackedCritical = unacked.some((a) => a.severity === 'critical')
  useEffect(() => {
    if (!alarmSound || unacked.length === 0) return
    beep(unackedCritical)
    const t = setInterval(() => beep(unackedCritical), 3000)
    return () => clearInterval(t)
  }, [alarmSound, unacked.length > 0, unackedCritical])

  // So are the charts.
  useEffect(() => {
    const json = JSON.stringify(charts)
    if (restored && json !== sentCharts.current) {
      sentCharts.current = json
      api.setCharts(charts)
    }
  }, [restored, charts])

  // Runs action, first offering to save unsaved changes.
  const guard = (action: () => Promise<void>) => {
    if (ws.dirty) setPending(() => action)
    else action().catch((e) => notify(errText(e)))
  }
  const loadWith = (fn: () => Promise<LoadResult | null>) => async () => {
    const r = await fn()
    if (r) await applyLoad(r)
  }
  const fileNew = () => guard(loadWith(api.newWorkspace))
  const fileOpen = () => guard(loadWith(api.openWorkspace))
  const fileOpenRecent = (path: string) => guard(loadWith(() => api.openWorkspacePath(path)))
  const fileSave = () => api.saveWorkspace().then((p) => p && notify(`Saved ${p}`, false)).catch((e) => notify(errText(e)))
  const fileSaveAs = () => api.saveWorkspaceAs().then((p) => p && notify(`Saved ${p}`, false)).catch((e) => notify(errText(e)))

  const logOpen = useRef(false)
  logOpen.current = dialog?.kind === 'applog'
  useEffect(() => events.onAppLog((e) => {
    const lv = e.level
    if (!logOpen.current && (lv === 'warn' || lv === 'error')) setLogUnseen((c) => ({ ...c, [lv]: c[lv] + 1 }))
  }), [])
  useEffect(() => { if (dialog?.kind === 'applog') setLogUnseen({ warn: 0, error: 0 }) }, [dialog?.kind])

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'F1' && !dialog) { e.preventDefault(); setDialog({ kind: 'help' }); return }
      if (menuStyle === 'modbuspoll' && !e.ctrlKey && !e.metaKey && !e.altKey && !dialog) {
        if (e.key === 'F3') { e.preventDefault(); setDialog({ kind: 'connections' }); return }
        if (e.key === 'F4') { e.preventDefault(); disconnectActive(); return }
        if (e.key === 'F8') { e.preventDefault(); editDefinition(); return }
      }
      if (!(e.ctrlKey || e.metaKey)) return
      const k = e.key.toLowerCase()
      if (k === 's') { e.preventDefault(); e.shiftKey ? fileSaveAs() : fileSave() }
      else if (k === 'o') { e.preventDefault(); fileOpen() }
      else if (k === 'n') { e.preventDefault(); fileNew() }
      else if (k === ',') { e.preventDefault(); setDialog({ kind: 'prefs' }) }
      else if (k === 'l' && e.shiftKey) { e.preventDefault(); setDialog({ kind: 'applog' }) }
    }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  })

  const addPoll = (p: Poll) => {
    setPolls((cur) => ({ ...cur, [p.config.id]: p }))
    setOrder((o) => (o.includes(p.config.id) ? o : [...o, p.config.id]))
    setActive(p.config.id)
  }
  const closePoll = async (id: string) => {
    await api.deletePoll(id)
    live.drop(id)
    setCharts((cur) => pruneCharts(cur, new Set(order.filter((x) => x !== id))))
    setPolls((cur) => {
      const next = { ...cur }
      delete next[id]
      return next
    })
    setOrder((o) => {
      const next = o.filter((x) => x !== id)
      if (active === id) setActive(next[Math.max(0, o.indexOf(id) - 1)] ?? charts.tabs[0]?.id ?? '')
      return next
    })
  }

  // ---- charts ----
  const unitOf = useCallback((s: ChartSeries) => polls[s.pollId]?.state.cells.find((c) => c.address === s.address)?.unit ?? '', [polls])
  const newChart = (series: ChartSeries[] = []): ChartDef => {
    const used = new Set(charts.tabs.map((t) => t.name))
    let n = charts.tabs.length + 1
    while (used.has(`Chart ${n}`)) n++
    return { id: newChartId(), name: `Chart ${n}`, rangeSec: 300, series }
  }
  const openNewChart = () => {
    const c = newChart()
    setCharts((cur) => ({ ...cur, tabs: [...cur.tabs, c] }))
    setActive(c.id)
    setDialog({ kind: 'addSeries', chartId: c.id })
  }
  const updateChart = useCallback((c: ChartDef) => setCharts((cur) => ({ ...cur, tabs: cur.tabs.map((t) => (t.id === c.id ? c : t)) })), [])
  const closeChart = (id: string) => {
    const i = charts.tabs.findIndex((t) => t.id === id)
    const rest = charts.tabs.filter((t) => t.id !== id)
    setCharts((cur) => ({ ...cur, tabs: cur.tabs.filter((t) => t.id !== id) }))
    if (active === id) setActive(rest[Math.max(0, i - 1)]?.id ?? order[order.length - 1] ?? '')
  }
  const onInline = useCallback((pollId: string, c: InlineChart) => setCharts((cur) => ({ ...cur, inline: { ...cur.inline, [pollId]: c } })), [])
  const onChartCell = useCallback((pollId: string, address: number, target: string) => {
    const unit = polls[pollId]?.state.cells.find((c) => c.address === address)?.unit ?? ''
    if (target === 'new') {
      const c = newChart(addSeries([], pollId, address, unitOf, unit))
      setCharts((cur) => ({ ...cur, tabs: [...cur.tabs, c] }))
      setActive(c.id)
      setView('tabs')
      return
    }
    setCharts((cur) => ({
      ...cur,
      tabs: cur.tabs.map((t) => (t.id === target ? { ...t, series: addSeries(t.series, pollId, address, unitOf, unit) } : t)),
    }))
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [polls, charts.tabs, unitOf])
  const chartTargets = useMemo(
    () => charts.tabs.map((t) => ({ id: t.id, name: t.name, full: t.series.length >= MAX_SERIES })),
    [charts.tabs],
  )

  // One click demo: simulator on :5020 + connection + two poll windows.
  const quickStart = async () => {
    try {
      const st = await api.simStatus()
      let port = st.config.port || defaultSim.port
      if (!st.running) {
        await api.simStart({ ...defaultSim })
        port = defaultSim.port
        await api.simWrite('holding', 0, [1200, 2400, 0x42f6, 0xe979, 65535, 7, 0, 0x1234, 500, 9600])
        await api.simWrite('coils', 0, [1, 0, 1, 1, 0, 0, 1, 0])
      }
      setSim(await api.simStatus())
      const conn = connections.find((c) => c.mode === 'tcp' && c.host === '127.0.0.1' && c.port === port)
        ?? await api.saveConnection({ ...blankConnection(), name: 'Local simulator', host: '127.0.0.1', port })
      // Label the demo windows with the bundled demo meter profile.
      const created = await api.applyProfile('bundled:appmodbus-demo-meter.json', conn.id, 1, 500)
      for (const p of created) {
        addPoll(p)
        await api.startPoll(p.config.id)
      }
      const live = created.find((p) => p.config.function === 4)
      if (live) setActive(live.config.id)
    } catch (e) {
      notify(errText(e))
    }
  }

  const connName = (id: string) => connections.find((c) => c.id === id)?.name ?? ''
  const activePoll = polls[active]
  const activeChart = charts.tabs.find((t) => t.id === active)
  const running = order.filter((id) => polls[id]?.state.running).length
  const onError = useCallback((m: string) => notify(m), [notify])
  const onSimStatus = useCallback((s: SimStatus) => setSim(s), [])
  const onInfo = useCallback((m: string) => notify(m, false), [notify])

  const renderPoll = (id: string) => {
    const p = polls[id]
    if (!p) return null
    return (
      <PollWindow
        key={id}
        poll={p}
        connectionName={connName(p.config.connectionId)}
        formats={formats}
        focused={view === 'tile' && id === active}
        onFocus={() => setActive(id)}
        onEdit={() => setDialog({ kind: 'poll', id })}
        onClose={() => closePoll(id)}
        onWrite={(cell) => setDialog({ kind: 'writeCell', pollId: id, cell })}
        onTag={(cell) => setDialog({ kind: 'tag', pollId: id, cell })}
        onError={onError}
        onInfo={onInfo}
        theme={theme}
        connections={connections}
        inline={charts.inline[id]}
        chartTargets={chartTargets}
        onInline={onInline}
        onChartCell={onChartCell}
      />
    )
  }

  const renderChart = (c: ChartDef) => (
    <ChartTab
      key={c.id}
      chart={c}
      polls={polls}
      connections={connections}
      theme={theme}
      focused={view === 'tile' && c.id === active}
      onFocus={() => setActive(c.id)}
      onChange={updateChart}
      onAdd={() => setDialog({ kind: 'addSeries', chartId: c.id })}
      onClose={() => closeChart(c.id)}
      onError={onError}
      onInfo={onInfo}
    />
  )
  const hasWindows = order.length + charts.tabs.length > 0

  const startAll = () => api.startAll().catch((e) => notify(errText(e)))
  const stopAll = () => api.stopAll()
  const fileProps = {
    ws, onNew: fileNew, onOpen: fileOpen, onOpenRecent: fileOpenRecent, onSave: fileSave, onSaveAs: fileSaveAs,
    onPreferences: () => setDialog({ kind: 'prefs' }),
  }
  const show = (kind: 'diag' | 'tests' | 'profiles' | 'data' | 'connections' | 'poll' | 'write' | 'about' | 'prefs' | 'help' | 'applog') => () => setDialog({ kind })
  const newEntries: MenuEntry[] = [
    { label: 'Poll window…', icon: <IconPlus />, onSelect: show('poll') },
    { label: 'Chart', icon: <IconChart />, onSelect: openNewChart },
  ]
  const pollingEntries: MenuEntry[] = [
    { label: 'Start all', icon: <IconPlay />, disabled: order.length === 0, onSelect: startAll },
    { label: 'Stop all', icon: <IconStop />, disabled: running === 0, onSelect: stopAll },
  ]
  const toolsEntries: MenuEntry[] = [
    { label: 'Diagnostics & scanning…', icon: <IconPulse />, onSelect: show('diag') },
    { label: 'Test sequences…', icon: <IconFlask />, onSelect: show('tests') },
    { type: 'sep' },
    { label: 'Device profiles…', icon: <IconBook />, onSelect: show('profiles') },
    { label: 'Data management…', icon: <IconDatabase />, onSelect: show('data') },
    { type: 'sep' },
    { label: 'Application log…', icon: <IconLog />, hint: isMac ? '⇧⌘L' : 'Ctrl+Shift+L', onSelect: show('applog') },
  ]
  const themeSub: MenuEntry = {
    type: 'sub', label: 'Theme', icon: theme === 'dark' ? <IconMoon /> : <IconSun />, items: [
      { label: 'Light', radio: true, checked: theme === 'light', onSelect: () => setTheme('light') },
      { label: 'Dark', radio: true, checked: theme === 'dark', onSelect: () => setTheme('dark') },
    ],
  }
  const menuStyleSub: MenuEntry = {
    type: 'sub', label: 'Menu style', items: [
      { label: 'Grouped toolbar', radio: true, checked: menuStyle === 'toolbar', onSelect: () => setMenuStyle('toolbar') },
      { label: 'Classic menu bar', radio: true, checked: menuStyle === 'menubar', onSelect: () => setMenuStyle('menubar') },
      { label: 'Modbus Poll style', radio: true, checked: menuStyle === 'modbuspoll', onSelect: () => setMenuStyle('modbuspoll') },
    ],
  }
  const viewEntries: MenuEntry[] = [
    {
      type: 'sub', label: 'Windows', icon: <IconTabs />, items: [
        { label: 'Tabbed', radio: true, checked: view === 'tabs', onSelect: () => setView('tabs') },
        { label: 'Tiled', radio: true, checked: view === 'tile', onSelect: () => setView('tile') },
      ],
    },
    {
      type: 'sub', label: 'Panels', icon: <IconList />, items: [
        { label: 'Alarms', checked: showAlarms, onSelect: () => setShowAlarms((v) => !v) },
        { label: 'Traffic log', checked: showTraffic, onSelect: () => setShowTraffic((v) => !v) },
        { label: 'Simulator', checked: showSim, onSelect: () => setShowSim((v) => !v) },
      ],
    },
    { type: 'sep' },
    themeSub,
    menuStyleSub,
  ]
  const barMenus: BarMenu[] = [
    { title: 'File', entries: fileMenuEntries({ ...fileProps, onExit: () => guard(async () => Quit()) }) },
    {
      title: 'Edit',
      entries: [
        { label: 'Connections…', icon: <IconPlug />, onSelect: show('connections') },
        { type: 'sub', label: 'New', icon: <IconPlus />, items: newEntries },
        { type: 'sep' },
        { label: 'Write…', icon: <IconPen />, disabled: connections.length === 0, onSelect: show('write') },
      ],
    },
    { title: 'View', entries: viewEntries },
    {
      title: 'Tools',
      entries: [{ type: 'sub', label: 'Polling', icon: <IconPlay />, items: pollingEntries }, { type: 'sep' }, ...toolsEntries],
    },
    {
      title: 'Help',
      entries: [
        { label: 'Modbus protocol guide…', icon: <IconHelp />, hint: 'F1', onSelect: show('help') },
        { type: 'sep' },
        {
          label: updates.state.status === 'available' && !updates.state.skipped ? `About & updates (${updates.state.release?.version} available)` : 'About & updates…',
          icon: <IconInfo />,
          onSelect: show('about'),
        },
      ],
    },
  ]
  const alarmsButton = (
    <button
      className={`btn has-badge${showAlarms ? ' on' : ''}`}
      title={unacked.length ? `${unacked.length} unacknowledged alarm(s)` : 'Alarms'}
      onClick={() => setShowAlarms((v) => !v)}
    >
      <IconBell />Alarms
      {unacked.length > 0 && <span className={`count-badge ${unackedCritical ? 'critical' : 'warning'}`}>{unacked.length}</span>}
    </button>
  )

  // ---- Modbus Poll-style menus: they act on the active poll window ----
  const ap = activePoll
  const apConn = ap && connections.find((x) => x.id === ap.config.connectionId)
  const savePoll = (patch: Partial<PollConfig>) => {
    if (ap) api.updatePoll({ ...ap.config, ...patch }).then(addPoll).catch((e) => notify(errText(e)))
  }
  const toggleActivePoll = () => {
    if (ap) (ap.state.running ? api.stopPoll(ap.config.id) : api.startPoll(ap.config.id)).catch((e) => notify(errText(e)))
  }
  const disconnectActive = () => {
    if (apConn?.connected) api.disconnect(apConn.id).catch((e) => notify(errText(e)))
  }
  const editDefinition = () => ap && setDialog({ kind: 'poll', id: ap.config.id })
  const write = (fn: number) => () => setDialog({ kind: 'write', fn })
  const diag = (section: 'stats' | 'ids' | 'map' | 'network' | 'device') => () => setDialog({ kind: 'diag', section })
  const noConn = connections.length === 0
  const displayDisabled = !ap || isBitFunction(ap.config.function)
  const displayFormats = formatGroups(formats, ap?.config.format ?? '', (id) => savePoll({ format: id }))
    .flatMap((g, i): MenuEntry[] => [...(i ? [{ type: 'sep' } as MenuEntry] : []), ...g.items])
    .map((e): MenuEntry => (e.type === 'sep' || e.type === 'header' || !displayDisabled ? e : { ...e, disabled: true }))
  const windowList: MenuEntry[] = [
    ...order.map((id, i): MenuEntry => ({ label: `${i + 1}  ${polls[id]?.config.name ?? id}`, radio: true, checked: active === id, onSelect: () => setActive(id) })),
    ...charts.tabs.map((t, j): MenuEntry => ({ label: `${order.length + j + 1}  ${t.name}`, radio: true, checked: active === t.id, onSelect: () => setActive(t.id) })),
  ]
  const mpMenus: BarMenu[] = [
    {
      title: 'File',
      entries: [
        { label: 'New poll window…', icon: <IconPlus />, onSelect: show('poll') },
        { type: 'sep' },
        ...fileMenuEntries({ ...fileProps, onExit: () => guard(async () => Quit()) }),
      ],
    },
    {
      title: 'Connection',
      entries: [
        { label: 'Connect…', icon: <IconPlug />, hint: 'F3', onSelect: show('connections') },
        { label: 'Disconnect', icon: <IconUnplug />, hint: 'F4', disabled: !apConn?.connected, onSelect: disconnectActive },
        { type: 'sep' },
        {
          type: 'sub', label: 'Connections', items: connections.length
            ? connections.map((x): MenuEntry => ({
                label: x.name, hint: x.summary, checked: x.connected,
                onSelect: () => (x.connected ? api.disconnect(x.id) : api.connect(x.id)).catch((e) => notify(errText(e))),
              }))
            : [{ label: 'No connections', disabled: true, onSelect: () => {} }],
        },
      ],
    },
    {
      title: 'Setup',
      entries: [
        { label: 'Read/Write definition…', icon: <IconGear />, hint: 'F8', disabled: !ap, onSelect: editDefinition },
        { label: 'Read/Write enabled', checked: !!ap?.state.running, disabled: !ap, onSelect: toggleActivePoll },
        { type: 'sub', label: 'All windows', icon: <IconPlay />, items: pollingEntries },
        { type: 'sep' },
        {
          label: 'Log to disk (REC)', checked: !!ap?.config.record, disabled: !ap,
          onSelect: () => ap && api.setPollRecording(ap.config.id, !ap.config.record)
            .then(() => addPoll({ ...ap, config: { ...ap.config, record: !ap.config.record } }))
            .catch((e) => notify(errText(e))),
        },
        { label: 'Data management…', icon: <IconDatabase />, onSelect: show('data') },
        { type: 'sep' },
        { label: 'Device profiles…', icon: <IconBook />, onSelect: show('profiles') },
        { label: 'Preferences…', hint: isMac ? '⌘,' : 'Ctrl+,', onSelect: show('prefs') },
      ],
    },
    {
      title: 'Functions',
      accel: 1,
      entries: [
        { label: 'Communication traffic', icon: <IconList />, checked: showTraffic, onSelect: () => setShowTraffic((v) => !v) },
        { type: 'sep' },
        { label: '05: Write single coil…', disabled: noConn, onSelect: write(5) },
        { label: '06: Write single register…', disabled: noConn, onSelect: write(6) },
        { label: '15: Write multiple coils…', disabled: noConn, onSelect: write(15) },
        { label: '16: Write multiple registers…', disabled: noConn, onSelect: write(16) },
        { label: '23: Read/Write multiple registers…', disabled: noConn, onSelect: write(23) },
        { type: 'sep' },
        { label: '08: Diagnostics…', onSelect: diag('device') },
        { label: '43/14: Read device identification…', onSelect: diag('device') },
        { type: 'sep' },
        {
          type: 'sub', label: 'Scan', icon: <IconPulse />, items: [
            { label: 'Slave IDs…', onSelect: diag('ids') },
            { label: 'Register map…', onSelect: diag('map') },
            { label: 'Modbus TCP network…', onSelect: diag('network') },
          ],
        },
        { label: 'Statistics…', onSelect: diag('stats') },
        { type: 'sep' },
        { label: 'Test center…', icon: <IconFlask />, onSelect: show('tests') },
      ],
    },
    {
      title: 'Display',
      entries: [
        ...displayFormats,
        { type: 'sep' },
        { label: 'PLC addresses (base 1)', radio: true, checked: !!ap?.config.plcAddressing, disabled: !ap, onSelect: () => savePoll({ plcAddressing: true }) },
        { label: 'Protocol addresses (base 0)', radio: true, checked: !!ap && !ap.config.plcAddressing, disabled: !ap, onSelect: () => savePoll({ plcAddressing: false }) },
      ],
    },
    {
      title: 'View',
      entries: [
        { label: 'Toolbar', checked: mpToolbar, onSelect: () => setMpToolbar((v) => !v) },
        { type: 'sep' },
        { label: 'Alarms', icon: <IconBell />, checked: showAlarms, onSelect: () => setShowAlarms((v) => !v) },
        { label: 'Simulator', checked: showSim, onSelect: () => setShowSim((v) => !v) },
        { label: 'Application log…', icon: <IconLog />, hint: isMac ? '⇧⌘L' : 'Ctrl+Shift+L', onSelect: show('applog') },
        { type: 'sep' },
        themeSub,
        menuStyleSub,
      ],
    },
    {
      title: 'Window',
      entries: [
        { label: 'Tabbed', radio: true, checked: view === 'tabs', onSelect: () => setView('tabs') },
        { label: 'Tile', radio: true, checked: view === 'tile', onSelect: () => setView('tile') },
        { type: 'sep' },
        { label: 'New chart', icon: <IconChart />, onSelect: openNewChart },
        ...(windowList.length ? [{ type: 'sep' } as MenuEntry, ...windowList] : []),
      ],
    },
    barMenus[barMenus.length - 1],
  ]

  return (
    <div className="app">
      <div>
      {menuStyle === 'modbuspoll' ? (
        <header className="mp-head">
          <div className="toolbar classic">
            <MenuBar menus={mpMenus} />
            <span className="tb-spacer" />
            <span className="ws-name" title={ws.path || 'Not saved yet'}>
              {ws.dirty && <span className="dirty" title="Unsaved changes" />}<span>{ws.name}</span>
            </span>
          </div>
          {mpToolbar && (
            <div className="mp-tb" role="toolbar">
              <button className="btn icon" title="New poll window" onClick={show('poll')}><IconPlus /></button>
              <button className="btn icon" title={`Open workspace (${isMac ? '⌘' : 'Ctrl+'}O)`} onClick={fileOpen}><IconFolder /></button>
              <button className="btn icon" title={`Save workspace (${isMac ? '⌘' : 'Ctrl+'}S)`} onClick={fileSave}><IconSave /></button>
              <span className="tb-sep" />
              <button className="btn icon" title="Connect (F3)" onClick={show('connections')}><IconPlug /></button>
              <button className="btn icon" title="Disconnect (F4)" disabled={!apConn?.connected} onClick={disconnectActive}><IconUnplug /></button>
              <span className="tb-sep" />
              <button className="btn icon" title="Start all" disabled={order.length === 0} onClick={startAll}><IconPlay /></button>
              <button className="btn icon" title="Stop all" disabled={running === 0} onClick={stopAll}><IconStop /></button>
              <button className="btn icon" title="Read/Write definition (F8)" disabled={!ap} onClick={editDefinition}><IconGear /></button>
              <span className="tb-sep" />
              {[5, 6, 15, 16, 23].map((fn) => (
                <button key={fn} className="btn fc" title={`Function ${fc2(fn)}`} disabled={noConn} onClick={write(fn)}>{fc2(fn)}</button>
              ))}
              <span className="tb-sep" />
              <button className={`btn icon${showTraffic ? ' on' : ''}`} title="Communication traffic" onClick={() => setShowTraffic((v) => !v)}><IconList /></button>
              <button className="btn icon" title="New chart" onClick={openNewChart}><IconChart /></button>
              <span className="tb-spacer" />
              {alarmsButton}
            </div>
          )}
        </header>
      ) : menuStyle === 'menubar' ? (
        <header className="toolbar classic">
          <MenuBar menus={barMenus} />
          <span className="ws-name" title={ws.path || 'Not saved yet'}>
            {ws.dirty && <span className="dirty" title="Unsaved changes" />}<span>{ws.name}</span>
          </span>
          <span className="tb-spacer" />
          <button className="btn icon" title="Start all" disabled={order.length === 0} onClick={startAll}><IconPlay /></button>
          <button className="btn icon" title="Stop all" disabled={running === 0} onClick={stopAll}><IconStop /></button>
          {alarmsButton}
        </header>
      ) : (
      <header className="toolbar">
        <div className="brand"><Logo />Modbus Studio</div>
        <FileMenu {...fileProps} />
        <span className="ws-name" title={ws.path || 'Not saved yet'}>
          {ws.dirty && <span className="dirty" title="Unsaved changes" />}<span>{ws.name}</span>
        </span>
        <span className="tb-sep" />
        <button className="btn" onClick={() => setDialog({ kind: 'connections' })}><IconPlug />Connection</button>
        <Dropdown label="New" icon={<IconPlus />} entries={newEntries} />
        <span className="tb-sep" />
        <button className="btn" disabled={order.length === 0} onClick={startAll}><IconPlay />Start all</button>
        <button className="btn" disabled={running === 0} onClick={stopAll}><IconStop />Stop all</button>
        <button className="btn" disabled={connections.length === 0} onClick={() => setDialog({ kind: 'write' })}><IconPen />Write</button>
        <span className="tb-sep" />
        <Dropdown label="Tools" icon={<IconWrench />} entries={toolsEntries} />
        <Dropdown label="View" icon={<IconEye />} entries={viewEntries} />
        <span className="tb-spacer" />
        {alarmsButton}
        <button className="btn icon" title="Modbus protocol guide (F1)" onClick={() => setDialog({ kind: 'help' })}><IconHelp /></button>
        <button className="btn icon" title="Preferences" onClick={() => setDialog({ kind: 'prefs' })}><IconGear /></button>
        <button
          className="btn icon has-badge"
          title={updates.state.status === 'available' ? `Update ${updates.state.release?.version} available` : 'About & updates'}
          onClick={() => setDialog({ kind: 'about' })}
        >
          <IconInfo />
          {updates.state.status === 'available' && !updates.state.skipped && <span className="badge" />}
        </button>
      </header>
      )}
      <UpdateBanner
        state={updates.state}
        progress={updates.progress}
        canInstall={info?.canInstall ?? false}
        onDetails={() => setDialog({ kind: 'about' })}
        onError={onError}
      />
      </div>

      <div className="main">
        <section className="workspace">
          {hasWindows && view === 'tabs' && (
            <nav className="tabs">
              {order.map((id) => {
                const p = polls[id]
                if (!p) return null
                return (
                  <div key={id} className={`tab${id === active ? ' active' : ''}`} onClick={() => setActive(id)} onDoubleClick={() => setDialog({ kind: 'poll', id })}>
                    <span className={`dot${p.state.lastError ? ' err' : p.state.running ? ' run' : ''}`} />
                    <span>{p.config.name}</span>
                    <span className="muted mono" style={{ fontSize: 11 }}>F{fc2(p.config.function)}·ID{p.config.slaveId}</span>
                    <button className="btn sm icon x" onClick={(e) => { e.stopPropagation(); closePoll(id) }} title="Close"><IconX /></button>
                  </div>
                )
              })}
              {charts.tabs.map((c) => (
                <div key={c.id} className={`tab chart-tab${c.id === active ? ' active' : ''}`} onClick={() => setActive(c.id)}>
                  <IconChart />
                  <span>{c.name}</span>
                  <span className="muted mono" style={{ fontSize: 11 }}>{c.series.length}</span>
                  <button className="btn sm icon x" onClick={(e) => { e.stopPropagation(); closeChart(c.id) }} title="Close"><IconX /></button>
                </div>
              ))}
              <button className="btn sm icon" style={{ marginBottom: 4 }} onClick={() => setDialog({ kind: 'poll' })} title="New poll window"><IconPlus /></button>
            </nav>
          )}
          <div className={`content${view === 'tile' && hasWindows ? ' tile' : ''}`}>
            {!hasWindows ? (
              <div className="empty">
                <div style={{ display: 'flex', justifyContent: 'center', marginBottom: 14 }}><Logo size={52} /></div>
                <h2>Welcome to Modbus Studio</h2>
                <p>A Modbus master for testing slaves over RTU and ASCII serial lines, Modbus TCP/UDP, and RTU/ASCII tunnelled over TCP or UDP.</p>
                <ol>
                  <li>Set up a <b>Connection</b> (serial port or IP address)</li>
                  <li>Add a <b>poll window</b> with slave ID, function, address and quantity</li>
                  <li>Press <b>Start</b>; double-click a value to write it</li>
                </ol>
                <div className="row">
                  <button className="btn outline" onClick={() => setDialog({ kind: 'connections' })}><IconPlug />New connection</button>
                  <button className="btn primary" onClick={quickStart}><IconBolt />Try with the built-in simulator</button>
                </div>
              </div>
            ) : view === 'tabs' ? (
              activeChart ? renderChart(activeChart) : activePoll && renderPoll(active)
            ) : (
              <>
                {order.map(renderPoll)}
                {charts.tabs.map(renderChart)}
              </>
            )}
          </div>
        </section>
        {showSim && <SimulatorPanel onClose={() => setShowSim(false)} onStatus={onSimStatus} />}
      </div>

      <TrafficPanel visible={showTraffic} onClose={() => setShowTraffic(false)} onError={onError} />
      <AlarmsPanel
        visible={showAlarms}
        alarms={alarms}
        polls={polls}
        sound={alarmSound}
        onSound={setAlarmSound}
        onClose={() => setShowAlarms(false)}
        onError={onError}
        onInfo={onInfo}
      />

      <footer className="statusbar">
        {connections.length === 0 && <span className="item">No connection</span>}
        {connections.map((c) => (
          <span className="item" key={c.id} title={c.error || undefined} style={{ cursor: 'pointer' }} onClick={() => setDialog({ kind: 'connections', id: c.id })}>
            <span className={`dot${c.connected ? ' ok' : c.error ? ' err' : ''}`} />
            {c.name} · <span className="mono">{c.summary}</span> · {c.connected ? 'Connected' : c.error ? c.error : 'Disconnected'}
          </span>
        ))}
        <span style={{ flex: 1 }} />
        {logUnseen.warn + logUnseen.error > 0 && (
          <span
            className={`item log-status ${logUnseen.error ? 'critical' : 'warning'}`}
            title="New warnings or errors in the application log"
            onClick={() => setDialog({ kind: 'applog' })}
          >
            <IconLog />{logUnseen.error ? `${logUnseen.error} new error(s)` : `${logUnseen.warn} new warning(s)`}
          </span>
        )}
        {alarms.some((a) => a.active) && (
          <span className={`item alarm-status ${alarms.some((a) => a.active && a.severity === 'critical') ? 'critical' : 'warning'}`} style={{ cursor: 'pointer' }} onClick={() => setShowAlarms(true)}>
            <IconBell />{alarms.filter((a) => a.active).length} active alarm(s)
          </span>
        )}
        {order.some((id) => polls[id]?.config.record && polls[id]?.state.running) && (
          <span className="item rec-status"><span className="rec-dot" />Recording {order.filter((id) => polls[id]?.config.record).length} window(s)</span>
        )}
        <span className="item">{running}/{order.length} polling</span>
        {info && <span className="item mono" style={{ cursor: 'pointer' }} onClick={() => setDialog({ kind: 'about' })}>v{info.version}</span>}
        <span className="item"><span className={`dot${sim?.running ? ' ok' : ''}`} />Simulator {sim?.running ? `on :${sim.config.port || sim.config.serialPort}` : 'off'}</span>
      </footer>

      {dialog?.kind === 'connections' && (
        <ConnectionsDialog connections={connections} initialId={dialog.id} onClose={() => setDialog(null)} />
      )}
      {dialog?.kind === 'poll' && (
        <PollDialog
          poll={dialog.id ? polls[dialog.id]?.config : undefined}
          connections={connections}
          formats={formats}
          onClose={() => setDialog(null)}
          onSaved={addPoll}
        />
      )}
      {dialog?.kind === 'writeCell' && polls[dialog.pollId] && (
        <WriteCellDialog poll={polls[dialog.pollId]} cell={dialog.cell} formats={formats} onClose={() => setDialog(null)} />
      )}
      {dialog?.kind === 'write' && (
        <ManualWriteDialog
          connections={connections}
          formats={formats}
          defaults={activePoll ? { connectionId: activePoll.config.connectionId, slaveId: activePoll.config.slaveId } : undefined}
          initialFunction={dialog.fn}
          onClose={() => setDialog(null)}
        />
      )}
      {dialog?.kind === 'tag' && polls[dialog.pollId] && (
        <TagDialog poll={polls[dialog.pollId]} cell={dialog.cell} onClose={() => setDialog(null)} />
      )}
      {dialog?.kind === 'profiles' && (
        <ProfilesDialog
          connections={connections}
          formats={formats}
          activePollId={active}
          onApplied={(created) => { created.forEach(addPoll); if (created[0]) setActive(created[0].config.id) }}
          onClose={() => setDialog(null)}
          onError={onError}
        />
      )}
      {dialog?.kind === 'addSeries' && charts.tabs.some((t) => t.id === dialog.chartId) && (
        <AddSeriesDialog
          chart={charts.tabs.find((t) => t.id === dialog.chartId)!}
          polls={polls}
          order={order}
          initialPollId={polls[active] ? active : undefined}
          onSave={(series) => updateChart({ ...charts.tabs.find((t) => t.id === dialog.chartId)!, series })}
          onClose={() => setDialog(null)}
        />
      )}
      {dialog?.kind === 'diag' && (
        <DiagnosticsDialog
          connections={connections}
          theme={theme}
          initialSection={dialog.section}
          onPolls={(created) => {
            created.forEach(addPoll)
            if (created[0]) { setActive(created[0].config.id); setView('tabs') }
            setDialog(null)
            notify(`Opened ${created.length} poll window(s)`, false)
          }}
          onClose={() => setDialog(null)}
          onInfo={onInfo}
        />
      )}
      {dialog?.kind === 'tests' && (
        <TestsDialog
          sequences={sequences}
          connections={connections}
          formats={formats}
          onChange={setSequences}
          onClose={() => setDialog(null)}
          onInfo={onInfo}
        />
      )}
      {dialog?.kind === 'data' && <DataDialog onClose={() => setDialog(null)} onInfo={(m) => notify(m, false)} />}
      {dialog?.kind === 'help' && <HelpDialog onClose={() => setDialog(null)} />}
      {dialog?.kind === 'applog' && <AppLogDialog onClose={() => setDialog(null)} onInfo={onInfo} />}
      {dialog?.kind === 'prefs' && (
        <PreferencesDialog
          theme={theme}
          onTheme={setTheme}
          menuStyle={menuStyle}
          onMenuStyle={setMenuStyle}
          alarmSound={alarmSound}
          onAlarmSound={setAlarmSound}
          onClose={() => setDialog(null)}
          onInfo={onInfo}
        />
      )}
      {dialog?.kind === 'about' && (
        <AboutDialog info={info} state={updates.state} progress={updates.progress} onClose={() => setDialog(null)} />
      )}
      {pending && (
        <UnsavedDialog
          name={ws.name}
          onCancel={() => setPending(null)}
          onDiscard={() => { const act = pending; setPending(null); act().catch((e) => notify(errText(e))) }}
          onSave={async () => {
            const act = pending
            setPending(null)
            try {
              if (await api.saveWorkspace()) await act()
            } catch (e) {
              notify(errText(e))
            }
          }}
        />
      )}
      {remap && <PortRemapDialog missing={remap.missingPorts} ports={remap.ports} onClose={() => setRemap(null)} onError={onError} />}
      {toast && <div className={`toast${toast.error ? ' error' : ''}`}>{toast.msg}</div>}
    </div>
  )
}
