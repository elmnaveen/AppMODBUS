import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { api, errText, events } from './api'
import { ConnectionsDialog, blankConnection } from './components/ConnectionsDialog'
import {
  IconBell, IconBolt, IconBook, IconChart, IconFlask, IconPulse, IconDatabase, IconInfo, IconList, IconMoon, IconPen, IconPlay, IconPlug, IconPlus, IconServer, IconStop, IconSun, IconTabs, IconTile, IconX,
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
import { FileMenu, PortRemapDialog, UnsavedDialog } from './components/Workspace'
import { ManualWriteDialog, WriteCellDialog } from './components/WriteDialogs'
import { useTheme } from './theme'
import { AddSeriesDialog, ChartTab } from './charts/ChartTab'
import { live } from './charts/live'
import {
  addSeries, emptyCharts, MAX_SERIES, newChartId, parseCharts, pruneCharts,
  type ChartDef, type ChartSeries, type ChartsDoc, type InlineChart,
} from './charts/model'
import { fc2, type Alarm, type AppInfo, type Sequence, type Cell, type LoadResult, type WorkspaceState, type ConnectionInfo, type FormatInfo, type Poll, type SimStatus } from './types'

type Dialog =
  | { kind: 'connections'; id?: string }
  | { kind: 'poll'; id?: string }
  | { kind: 'writeCell'; pollId: string; cell: Cell }
  | { kind: 'write' }
  | { kind: 'about' }
  | { kind: 'tag'; pollId: string; cell: Cell }
  | { kind: 'profiles' }
  | { kind: 'data' }
  | { kind: 'addSeries'; chartId: string }
  | { kind: 'diag' }
  | { kind: 'tests' }
  | null

export default function App() {
  const [theme, toggleTheme] = useTheme()
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

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (!(e.ctrlKey || e.metaKey)) return
      const k = e.key.toLowerCase()
      if (k === 's') { e.preventDefault(); e.shiftKey ? fileSaveAs() : fileSave() }
      else if (k === 'o') { e.preventDefault(); fileOpen() }
      else if (k === 'n') { e.preventDefault(); fileNew() }
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

  return (
    <div className="app">
      <div>
      <header className="toolbar">
        <div className="brand"><span className="brand-mark">MB</span>AppMODBUS</div>
        <FileMenu ws={ws} onNew={fileNew} onOpen={fileOpen} onOpenRecent={fileOpenRecent} onSave={fileSave} onSaveAs={fileSaveAs} />
        <span className="ws-name" title={ws.path || 'Not saved yet'}>
          {ws.dirty && <span className="dirty" title="Unsaved changes" />}<span>{ws.name}</span>
        </span>
        <span className="tb-sep" />
        <button className="btn" onClick={() => setDialog({ kind: 'connections' })}><IconPlug />Connection</button>
        <button className="btn" onClick={() => setDialog({ kind: 'poll' })}><IconPlus />New poll</button>
        <button className="btn" onClick={openNewChart}><IconChart />New chart</button>
        <span className="tb-sep" />
        <button className="btn" disabled={order.length === 0} onClick={() => api.startAll().catch((e) => notify(errText(e)))}><IconPlay />Start all</button>
        <button className="btn" disabled={running === 0} onClick={() => api.stopAll()}><IconStop />Stop all</button>
        <button className="btn" disabled={connections.length === 0} onClick={() => setDialog({ kind: 'write' })}><IconPen />Write</button>
        <button className="btn" onClick={() => setDialog({ kind: 'profiles' })}><IconBook />Profiles</button>
        <button className="btn" onClick={() => setDialog({ kind: 'data' })}><IconDatabase />Data</button>
        <button className="btn" onClick={() => setDialog({ kind: 'diag' })}><IconPulse />Diagnostics</button>
        <button className="btn" onClick={() => setDialog({ kind: 'tests' })}><IconFlask />Tests</button>
        <span className="tb-sep" />
        <button className={`btn icon${view === 'tabs' ? ' on' : ''}`} title="Tabbed view" onClick={() => setView('tabs')}><IconTabs /></button>
        <button className={`btn icon${view === 'tile' ? ' on' : ''}`} title="Tile all windows" onClick={() => setView('tile')}><IconTile /></button>
        <span className="tb-spacer" />
        <button
          className={`btn has-badge${showAlarms ? ' on' : ''}`}
          title={unacked.length ? `${unacked.length} unacknowledged alarm(s)` : 'Alarms'}
          onClick={() => setShowAlarms((v) => !v)}
        >
          <IconBell />Alarms
          {unacked.length > 0 && <span className={`count-badge ${unackedCritical ? 'critical' : 'warning'}`}>{unacked.length}</span>}
        </button>
        <button className={`btn${showTraffic ? ' on' : ''}`} onClick={() => setShowTraffic((v) => !v)}><IconList />Traffic</button>
        <button className={`btn${showSim ? ' on' : ''}`} onClick={() => setShowSim((v) => !v)}><IconServer />Simulator</button>
        <button className="btn icon" title={theme === 'dark' ? 'Light mode' : 'Dark mode'} onClick={toggleTheme}>{theme === 'dark' ? <IconSun /> : <IconMoon />}</button>
        <button
          className="btn icon has-badge"
          title={updates.state.status === 'available' ? `Update ${updates.state.release?.version} available` : 'About & updates'}
          onClick={() => setDialog({ kind: 'about' })}
        >
          <IconInfo />
          {updates.state.status === 'available' && !updates.state.skipped && <span className="badge" />}
        </button>
      </header>
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
                <div className="brand-mark" style={{ width: 44, height: 44, fontSize: 16, margin: '0 auto 14px', borderRadius: 10 }}>MB</div>
                <h2>Welcome to AppMODBUS</h2>
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
