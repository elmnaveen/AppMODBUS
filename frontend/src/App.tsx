import { useCallback, useEffect, useState } from 'react'
import { api, errText, events } from './api'
import { ConnectionsDialog, blankConnection } from './components/ConnectionsDialog'
import {
  IconBolt, IconBook, IconInfo, IconList, IconMoon, IconPen, IconPlay, IconPlug, IconPlus, IconServer, IconStop, IconSun, IconTabs, IconTile, IconX,
} from './components/Icons'
import { PollDialog } from './components/PollDialog'
import { PollWindow } from './components/PollWindow'
import { SimulatorPanel, defaultSim } from './components/SimulatorPanel'
import { TrafficPanel } from './components/TrafficPanel'
import { AboutDialog, UpdateBanner, useUpdates } from './components/Updates'
import { ProfilesDialog } from './components/Profiles'
import { TagDialog } from './components/TagDialog'
import { FileMenu, PortRemapDialog, UnsavedDialog } from './components/Workspace'
import { ManualWriteDialog, WriteCellDialog } from './components/WriteDialogs'
import { useTheme } from './theme'
import { fc2, type AppInfo, type Cell, type LoadResult, type WorkspaceState, type ConnectionInfo, type FormatInfo, type Poll, type SimStatus } from './types'

type Dialog =
  | { kind: 'connections'; id?: string }
  | { kind: 'poll'; id?: string }
  | { kind: 'writeCell'; pollId: string; cell: Cell }
  | { kind: 'write' }
  | { kind: 'about' }
  | { kind: 'tag'; pollId: string; cell: Cell }
  | { kind: 'profiles' }
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

  const notify = useCallback((msg: string, error = true) => {
    setToast({ msg, error })
    setTimeout(() => setToast((t) => (t?.msg === msg ? null : t)), 4000)
  }, [])

  useEffect(() => {
    api.appInfo().then(setInfo)
    api.formats().then(setFormats)
    api.restoreSession().then(applyLoad).finally(() => setRestored(true))
    const offs = [
      events.onPoll((p) => setPolls((cur) => ({ ...cur, [p.config.id]: p }))),
      events.onConnections(setConnections),
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
    setActive(ps.some((p) => p.config.id === r.layout.activeId) ? r.layout.activeId : ps[0]?.config.id ?? '')
    if (r.warnings.length) notify(`Some items could not be loaded: ${r.warnings.join('; ')}`)
    if (r.missingPorts.length) setRemap(r)
  }

  // The layout is saved with the workspace.
  useEffect(() => {
    if (restored) api.setLayout({ view, activeId: active, showTraffic, showSimulator: showSim })
  }, [restored, view, active, showTraffic, showSim])

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
    setPolls((cur) => {
      const next = { ...cur }
      delete next[id]
      return next
    })
    setOrder((o) => {
      const next = o.filter((x) => x !== id)
      if (active === id) setActive(next[Math.max(0, o.indexOf(id) - 1)] ?? '')
      return next
    })
  }

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
  const running = order.filter((id) => polls[id]?.state.running).length
  const onError = useCallback((m: string) => notify(m), [notify])
  const onSimStatus = useCallback((s: SimStatus) => setSim(s), [])

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
      />
    )
  }

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
        <span className="tb-sep" />
        <button className="btn" disabled={order.length === 0} onClick={() => api.startAll().catch((e) => notify(errText(e)))}><IconPlay />Start all</button>
        <button className="btn" disabled={running === 0} onClick={() => api.stopAll()}><IconStop />Stop all</button>
        <button className="btn" disabled={connections.length === 0} onClick={() => setDialog({ kind: 'write' })}><IconPen />Write</button>
        <button className="btn" onClick={() => setDialog({ kind: 'profiles' })}><IconBook />Profiles</button>
        <span className="tb-sep" />
        <button className={`btn icon${view === 'tabs' ? ' on' : ''}`} title="Tabbed view" onClick={() => setView('tabs')}><IconTabs /></button>
        <button className={`btn icon${view === 'tile' ? ' on' : ''}`} title="Tile all windows" onClick={() => setView('tile')}><IconTile /></button>
        <span className="tb-spacer" />
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
          {order.length > 0 && view === 'tabs' && (
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
              <button className="btn sm icon" style={{ marginBottom: 4 }} onClick={() => setDialog({ kind: 'poll' })} title="New poll window"><IconPlus /></button>
            </nav>
          )}
          <div className={`content${view === 'tile' && order.length ? ' tile' : ''}`}>
            {order.length === 0 ? (
              <div className="empty">
                <div className="brand-mark" style={{ width: 44, height: 44, fontSize: 16, margin: '0 auto 14px', borderRadius: 10 }}>MB</div>
                <h2>Welcome to AppMODBUS</h2>
                <p>A Modbus master for testing slaves over RTU, TCP/IP and RTU over TCP/UDP.</p>
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
              activePoll && renderPoll(active)
            ) : (
              order.map(renderPoll)
            )}
          </div>
        </section>
        {showSim && <SimulatorPanel onClose={() => setShowSim(false)} onStatus={onSimStatus} />}
      </div>

      <TrafficPanel visible={showTraffic} onClose={() => setShowTraffic(false)} onError={onError} />

      <footer className="statusbar">
        {connections.length === 0 && <span className="item">No connection</span>}
        {connections.map((c) => (
          <span className="item" key={c.id} title={c.error || undefined} style={{ cursor: 'pointer' }} onClick={() => setDialog({ kind: 'connections', id: c.id })}>
            <span className={`dot${c.connected ? ' ok' : c.error ? ' err' : ''}`} />
            {c.name} · <span className="mono">{c.summary}</span> · {c.connected ? 'Connected' : c.error ? c.error : 'Disconnected'}
          </span>
        ))}
        <span style={{ flex: 1 }} />
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
