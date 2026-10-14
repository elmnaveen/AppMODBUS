import { useEffect, useRef, useState } from 'react'
import { api, errText } from '../api'
import type { MissingPort, WorkspaceState } from '../types'
import { IconChevron, IconFile, IconFolder, IconPlus, IconSave } from './Icons'
import { Modal } from './Modal'

const isMac = navigator.platform.toUpperCase().includes('MAC')
const mod = isMac ? '⌘' : 'Ctrl+'

interface MenuProps {
  ws: WorkspaceState
  onNew: () => void
  onOpen: () => void
  onOpenRecent: (path: string) => void
  onSave: () => void
  onSaveAs: () => void
}

/** The File dropdown in the toolbar. */
export function FileMenu({ ws, onNew, onOpen, onOpenRecent, onSave, onSaveAs }: MenuProps) {
  const [open, setOpen] = useState(false)
  const ref = useRef<HTMLDivElement>(null)
  useEffect(() => {
    if (!open) return
    const close = (e: MouseEvent) => !ref.current?.contains(e.target as Node) && setOpen(false)
    window.addEventListener('mousedown', close)
    return () => window.removeEventListener('mousedown', close)
  }, [open])
  const run = (fn: () => void) => () => { setOpen(false); fn() }
  return (
    <div className="dropdown" ref={ref}>
      <button className={`btn${open ? ' on' : ''}`} onClick={() => setOpen((o) => !o)}><IconFile />File<IconChevron /></button>
      {open && (
        <div className="ctx menu-pop">
          <button onClick={run(onNew)}><IconPlus />New workspace<kbd>{mod}N</kbd></button>
          <button onClick={run(onOpen)}><IconFolder />Open…<kbd>{mod}O</kbd></button>
          <button onClick={run(onSave)}><IconSave />Save<kbd>{mod}S</kbd></button>
          <button onClick={run(onSaveAs)}><span className="ico-pad" />Save as…<kbd>{isMac ? '⇧⌘S' : 'Ctrl+Shift+S'}</kbd></button>
          <hr />
          <div className="grp">Recent</div>
          {ws.recent.length === 0 && <div className="muted" style={{ padding: '4px 10px 6px' }}>No recent workspaces</div>}
          {ws.recent.map((p) => (
            <button key={p} onClick={run(() => onOpenRecent(p))} title={p}>
              <span className="recent-name">{p.split(/[\\/]/).pop()?.replace(/\.mbw$/i, '')}</span>
              <span className="recent-path">{p}</span>
            </button>
          ))}
        </div>
      )}
    </div>
  )
}

/** Asks what to do with unsaved changes. Resolves to whether to continue. */
export function UnsavedDialog({ name, onSave, onDiscard, onCancel }: { name: string; onSave: () => void; onDiscard: () => void; onCancel: () => void }) {
  return (
    <Modal
      title="Unsaved changes"
      onClose={onCancel}
      width={460}
      footer={
        <>
          <button className="btn outline" onClick={onCancel}>Cancel</button>
          <button className="btn outline" onClick={onDiscard}>Don't save</button>
          <button className="btn primary" onClick={onSave}>Save</button>
        </>
      }
    >
      <p style={{ margin: 0 }}>Do you want to save the changes to <b>{name}</b>?</p>
      <p className="muted" style={{ margin: '8px 0 0' }}>Your changes will be lost if you don't save them.</p>
    </Modal>
  )
}

/** Lets the user replace serial ports that do not exist on this machine. */
export function PortRemapDialog({ missing, ports, onClose, onError }: { missing: MissingPort[]; ports: string[]; onClose: () => void; onError: (m: string) => void }) {
  const [choice, setChoice] = useState<Record<string, string>>(() => Object.fromEntries(missing.map((m) => [m.connectionId || 'sim', ''])))
  const apply = async () => {
    try {
      for (const m of missing) {
        const port = choice[m.connectionId || 'sim']
        if (port) await api.remapSerialPort(m.connectionId, port)
      }
      onClose()
    } catch (e) {
      onError(errText(e))
    }
  }
  return (
    <Modal
      title="Serial ports not found"
      onClose={onClose}
      width={560}
      footer={
        <>
          <button className="btn outline" onClick={onClose}>Keep as is</button>
          <button className="btn primary" onClick={apply}>Apply</button>
        </>
      }
    >
      <p style={{ marginTop: 0 }} className="muted">
        This workspace uses serial ports that are not present on this computer. Choose a replacement, or keep the saved name (e.g. if the adapter is not plugged in yet).
      </p>
      <div className="form" style={{ gridTemplateColumns: '1fr 1fr' }}>
        {missing.map((m) => (
          <div key={m.connectionId || 'sim'} style={{ display: 'contents' }}>
            <label className="lbl"><b style={{ color: 'var(--text)' }}>{m.name}</b> <span className="mono">({m.port})</span></label>
            <select value={choice[m.connectionId || 'sim']} onChange={(e) => setChoice((c) => ({ ...c, [m.connectionId || 'sim']: e.target.value }))}>
              <option value="">Keep {m.port}</option>
              {ports.map((p) => <option key={p} value={p}>{p}</option>)}
            </select>
          </div>
        ))}
        {ports.length === 0 && <div className="alert full">No serial ports are currently detected. Plug in the adapter and reopen the workspace, or keep the saved names.</div>}
      </div>
    </Modal>
  )
}
