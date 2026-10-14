import { useEffect, useState, type ReactNode } from 'react'
import { api, errText } from '../api'
import type { MenuStyle, Theme } from '../theme'
import type { DataSettings, ProfileFolders, UpdateSettings } from '../types'
import { saveAlarmSound } from './AlarmsPanel'
import { IconBell, IconBook, IconDatabase, IconDownload, IconEye, IconFolder, IconSave } from './Icons'
import { Modal } from './Modal'

type Category = 'appearance' | 'alarms' | 'updates' | 'data' | 'profiles'

const CATEGORIES: { id: Category; label: string; icon: ReactNode }[] = [
  { id: 'appearance', label: 'Appearance', icon: <IconEye /> },
  { id: 'alarms', label: 'Alarms', icon: <IconBell /> },
  { id: 'updates', label: 'Updates', icon: <IconDownload /> },
  { id: 'data', label: 'Recording data', icon: <IconDatabase /> },
  { id: 'profiles', label: 'Device profiles', icon: <IconBook /> },
]

interface Props {
  theme: Theme
  onTheme: (t: Theme) => void
  menuStyle: MenuStyle
  onMenuStyle: (m: MenuStyle) => void
  alarmSound: boolean
  onAlarmSound: (on: boolean) => void
  onClose: () => void
  onInfo: (m: string) => void
}

/** Preferences window: every per-user setting, grouped by category. */
export function PreferencesDialog(props: Props) {
  const [cat, setCat] = useState<Category>('appearance')
  return (
    <Modal title="Preferences" onClose={props.onClose} width={760} footer={<button className="btn primary" onClick={props.onClose}>Done</button>}>
      <div className="prefs">
        <nav className="prefs-nav">
          {CATEGORIES.map((c) => (
            <button key={c.id} className={cat === c.id ? 'on' : ''} onClick={() => setCat(c.id)}>{c.icon}{c.label}</button>
          ))}
        </nav>
        <div className="prefs-page">
          {cat === 'appearance' && <AppearancePage {...props} />}
          {cat === 'alarms' && <AlarmsPage {...props} />}
          {cat === 'updates' && <UpdatesPage />}
          {cat === 'data' && <DataPage onInfo={props.onInfo} />}
          {cat === 'profiles' && <ProfilesPage />}
        </div>
      </div>
    </Modal>
  )
}

function Seg<T extends string>({ value, options, onChange }: { value: T | undefined; options: [T, string][]; onChange: (v: T) => void }) {
  return (
    <div className="seg">
      {options.map(([v, label]) => <button key={v} className={value === v ? 'on' : ''} onClick={() => onChange(v)}>{label}</button>)}
    </div>
  )
}

function AppearancePage({ theme, onTheme, menuStyle, onMenuStyle }: Props) {
  return (
    <>
      <h4>Appearance</h4>
      <div className="form">
        <label className="lbl">Theme</label>
        <Seg value={theme} options={[['light', 'Light'], ['dark', 'Dark']]} onChange={onTheme} />
        <label className="lbl">Menu style</label>
        <Seg value={menuStyle} options={[['toolbar', 'Grouped toolbar'], ['menubar', 'Classic menu bar'], ['modbuspoll', 'Modbus Poll style']]} onChange={onMenuStyle} />
        <span className="hint">
          {menuStyle === 'toolbar'
            ? 'Buttons with File, New, Tools and View drop-downs.'
            : menuStyle === 'menubar'
              ? 'A File / Edit / View / Tools / Help menu bar, as in most Windows programs. Alt+letter opens a menu.'
              : 'Modbus Poll layout: File, Connection, Setup, Functions, Display, View, Window and Help menus with an icon toolbar and 05 / 06 / 15 / 16 / 23 write buttons. F3 connect, F4 disconnect, F8 read/write definition.'}
        </span>
      </div>
    </>
  )
}

function AlarmsPage({ alarmSound, onAlarmSound }: Props) {
  return (
    <>
      <h4>Alarms</h4>
      <div className="form">
        <label className="lbl">Sound</label>
        <label className="check">
          <input type="checkbox" checked={alarmSound} onChange={(e) => { saveAlarmSound(e.target.checked); onAlarmSound(e.target.checked) }} />
          Beep while there are unacknowledged alarms
        </label>
        <span className="hint">Alarm limits are set per register (right-click a cell → Tag) and saved in the workspace.</span>
      </div>
    </>
  )
}

function UpdatesPage() {
  const [s, setS] = useState<UpdateSettings | null>(null)
  const [error, setError] = useState('')
  useEffect(() => { api.updateSettings().then(setS).catch((e) => setError(errText(e))) }, [])
  const save = async (channel: string, auto: boolean, recheck: boolean) => {
    try {
      setError('')
      setS(await api.setUpdateSettings(channel, auto))
      if (recheck) api.checkForUpdates().catch(() => { /* shown in About */ })
    } catch (e) {
      setError(errText(e))
    }
  }
  return (
    <>
      <h4>Updates</h4>
      <div className="form">
        <label className="lbl">Update channel</label>
        <Seg value={s?.updateChannel} options={[['stable', 'Stable'], ['beta', 'Beta (pre-releases)']]} onChange={(c) => s && save(c, s.autoCheckUpdates, true)} />
        <label className="lbl">Automatic check</label>
        <label className="check">
          <input type="checkbox" checked={s?.autoCheckUpdates ?? true} onChange={(e) => s && save(s.updateChannel, e.target.checked, false)} />
          Check for updates when Modbus Studio starts
        </label>
        <span className="hint">Help → About &amp; updates shows the current status and installs updates.</span>
        {error && <div className="alert full">{error}</div>}
      </div>
    </>
  )
}

function DataPage({ onInfo }: { onInfo: (m: string) => void }) {
  const [s, setS] = useState<DataSettings | null>(null)
  const [retention, setRetention] = useState('30')
  const [maxGB, setMaxGB] = useState('2')
  const [error, setError] = useState('')
  const apply = (d: DataSettings) => {
    setS(d)
    setRetention(String(d.retentionDays))
    setMaxGB(d.maxStorageMB ? String(+(d.maxStorageMB / 1024).toFixed(2)) : '0')
  }
  useEffect(() => { api.dataSettings().then(apply).catch((e) => setError(errText(e))) }, [])
  const run = async (fn: () => Promise<void>) => {
    try {
      setError('')
      await fn()
    } catch (e) {
      setError(errText(e))
    }
  }
  const saveLimits = () => run(async () => {
    const days = Number(retention)
    const gb = Number(maxGB)
    if (!Number.isInteger(days) || days < 0) throw new Error('Retention must be a whole number of days (0 = forever)')
    if (!Number.isFinite(gb) || gb < 0) throw new Error('Maximum size must be a number of GB (0 = no limit)')
    apply(await api.setDataLimits(days, Math.round(gb * 1024)))
    onInfo('Data limits saved')
  })
  return (
    <>
      <h4>Recording data</h4>
      <div className="form">
        <label className="lbl">Folder</label>
        <div className="inline">
          <span className="mono path" title={s?.folder}>{s?.folder}</span>
          <button className="btn sm outline" onClick={() => run(async () => apply(await api.chooseRecordingsFolder()))}><IconFolder />Change…</button>
          {s && s.folder !== s.defaultFolder && <button className="btn sm" onClick={() => run(async () => apply(await api.resetRecordingsFolder()))}>Default</button>}
        </div>
        <span className="hint">New recordings go to this folder; existing ones stay where they are.</span>
        <label className="lbl">Keep data for</label>
        <div className="inline"><input type="number" min={0} value={retention} onChange={(e) => setRetention(e.target.value)} style={{ width: 100 }} /> <span className="muted">days (0 = forever)</span></div>
        <label className="lbl">Maximum size</label>
        <div className="inline"><input type="number" min={0} step={0.5} value={maxGB} onChange={(e) => setMaxGB(e.target.value)} style={{ width: 100 }} /> <span className="muted">GB (0 = no limit)</span></div>
        <span className="hint">When full, the oldest days are deleted first. Tools → Data management lists and exports recordings.</span>
        <div className="full"><button className="btn sm primary" onClick={saveLimits}><IconSave />Save limits</button></div>
        {error && <div className="alert full">{error}</div>}
      </div>
    </>
  )
}

function ProfilesPage() {
  const [f, setF] = useState<ProfileFolders | null>(null)
  const [error, setError] = useState('')
  const refresh = () => api.profileFolders().then(setF).catch((e) => setError(errText(e)))
  useEffect(() => { refresh() }, [])
  const run = async (fn: () => Promise<unknown>) => {
    try {
      setError('')
      await fn()
      await refresh()
    } catch (e) {
      setError(errText(e))
    }
  }
  return (
    <>
      <h4>Device profiles</h4>
      <div className="form">
        <label className="lbl">Personal folder</label>
        <span className="mono path" title={f?.user}>{f?.user}</span>
        <label className="lbl">Shared folder</label>
        <div className="inline">
          {f?.shared ? <span className="mono path" title={f.shared}>{f.shared}</span> : <span className="muted">Not used</span>}
          <button className="btn sm outline" onClick={() => run(api.chooseSharedProfileFolder)}><IconFolder />{f?.shared ? 'Change…' : 'Choose…'}</button>
          {f?.shared && <button className="btn sm" onClick={() => run(api.clearSharedProfileFolder)}>Stop using</button>}
        </div>
        <span className="hint">A shared folder (e.g. a network drive) lets a team use the same device profiles.</span>
        {error && <div className="alert full">{error}</div>}
      </div>
    </>
  )
}
