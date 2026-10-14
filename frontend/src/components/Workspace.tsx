import { useState } from 'react'
import { api, errText } from '../api'
import type { MissingPort, WorkspaceState } from '../types'
import { IconFile, IconFolder, IconGear, IconPlus, IconSave } from './Icons'
import { Dropdown, type MenuEntry } from './Menu'
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
  onPreferences: () => void
  /** Shown as File → Exit when set (classic menu bar). */
  onExit?: () => void
}

/** The File menu, shared by the toolbar drop-down and the classic menu bar. */
export function fileMenuEntries({ ws, onNew, onOpen, onOpenRecent, onSave, onSaveAs, onPreferences, onExit }: MenuProps): MenuEntry[] {
  const recent: MenuEntry[] = ws.recent.length === 0
    ? [{ label: 'No recent workspaces', disabled: true, onSelect: () => {} }]
    : ws.recent.map((p, i) => ({ label: `${i + 1}  ${p.split(/[\\/]/).pop()?.replace(/\.mbw$/i, '')}`, title: p, onSelect: () => onOpenRecent(p) }))
  const entries: MenuEntry[] = [
    { label: 'New workspace', icon: <IconPlus />, hint: `${mod}N`, onSelect: onNew },
    { label: 'Open…', icon: <IconFolder />, hint: `${mod}O`, onSelect: onOpen },
    { type: 'sub', label: 'Open recent', items: recent },
    { type: 'sep' },
    { label: 'Save', icon: <IconSave />, hint: `${mod}S`, onSelect: onSave },
    { label: 'Save as…', hint: isMac ? '⇧⌘S' : 'Ctrl+Shift+S', onSelect: onSaveAs },
    { type: 'sep' },
    { label: 'Preferences…', icon: <IconGear />, hint: `${mod},`, onSelect: onPreferences },
  ]
  if (onExit) entries.push({ type: 'sep' }, { label: 'Exit', hint: isMac ? '⌘Q' : 'Alt+F4', onSelect: onExit })
  return entries
}

/** The File dropdown in the toolbar. */
export function FileMenu(props: MenuProps) {
  return <Dropdown label="File" icon={<IconFile />} entries={fileMenuEntries(props)} />
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
