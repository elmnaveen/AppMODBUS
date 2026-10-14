import { useEffect, useState, type ReactNode } from 'react'
import { api, errText, events } from '../api'
import type { AppInfo, Release, UpdateProgress, UpdateSettings, UpdateState } from '../types'
import { IconDownload, IconExternal, IconRefresh, IconX } from './Icons'
import { Modal } from './Modal'

/** Live updater state shared by the banner, toolbar badge and dialog. */
export function useUpdates() {
  const [state, setState] = useState<UpdateState>({ status: 'idle', release: null, error: '', checkedAt: 0, skipped: false })
  const [progress, setProgress] = useState<UpdateProgress | null>(null)
  useEffect(() => {
    api.updateState().then(setState)
    const offs = [
      events.onUpdateState((s) => {
        setState(s)
        if (s.status !== 'downloading') setProgress(null)
      }),
      events.onUpdateProgress(setProgress),
    ]
    return () => offs.forEach((off) => off())
  }, [])
  return { state, progress }
}

const busy = (s: UpdateState) => s.status === 'downloading' || s.status === 'installing'

const fmtBytes = (n: number) => (n >= 1 << 20 ? `${(n / (1 << 20)).toFixed(1)} MB` : `${Math.round(n / 1024)} KB`)

function ProgressBar({ state, progress }: { state: UpdateState; progress: UpdateProgress | null }) {
  if (state.status === 'installing') return <div className="upd-progress"><div className="bar indeterminate" /><span>Verifying and installing…</span></div>
  if (state.status !== 'downloading') return null
  const pct = progress && progress.total > 0 ? Math.min(100, (100 * progress.done) / progress.total) : 0
  return (
    <div className="upd-progress">
      <div className="bar"><div style={{ width: `${pct}%` }} /></div>
      <span className="mono">{progress ? `${fmtBytes(progress.done)}${progress.total ? ` / ${fmtBytes(progress.total)}` : ''}` : 'Starting…'}</span>
    </div>
  )
}

// ---------- banner ----------

interface BannerProps {
  state: UpdateState
  progress: UpdateProgress | null
  canInstall: boolean
  onDetails: () => void
  onError: (m: string) => void
}

export function UpdateBanner({ state, progress, canInstall, onDetails, onError }: BannerProps) {
  const [hidden, setHidden] = useState('')
  const rel = state.release
  const show = rel && !state.skipped && hidden !== rel.version &&
    (state.status === 'available' || busy(state) || (state.status === 'error' && state.release))
  if (!show || !rel) return null
  return (
    <div className="upd-banner">
      <IconDownload />
      <span>
        <b>AppMODBUS {rel.version}</b> {rel.prerelease && <span className="pill beta">Beta</span>} is available
      </span>
      {busy(state) ? (
        <ProgressBar state={state} progress={progress} />
      ) : (
        <>
          {state.status === 'error' && <span className="err-text upd-err" title={state.error}>Update failed: {state.error}</span>}
          <span style={{ flex: 1 }} />
          <button className="btn sm" onClick={onDetails}>What's new</button>
          {canInstall ? (
            <button className="btn sm primary" onClick={() => api.installUpdate().catch((e) => onError(errText(e)))}>Install &amp; restart</button>
          ) : (
            <button className="btn sm primary" onClick={() => api.openReleasePage()}><IconExternal />Download</button>
          )}
          <button className="btn sm" onClick={() => api.skipVersion(rel.version)}>Skip this version</button>
          <button className="btn sm icon" title="Remind me later" onClick={() => setHidden(rel.version)}><IconX /></button>
        </>
      )}
    </div>
  )
}

// ---------- release notes (tiny markdown subset) ----------

function inline(text: string): ReactNode[] {
  return text.split(/(\*\*[^*]+\*\*|`[^`]+`)/g).map((part, i) =>
    part.startsWith('**') && part.endsWith('**') ? <b key={i}>{part.slice(2, -2)}</b>
      : part.startsWith('`') && part.endsWith('`') ? <code key={i}>{part.slice(1, -1)}</code>
        : part,
  )
}

function Notes({ text }: { text: string }) {
  if (!text.trim()) return <p className="muted">No release notes.</p>
  const out: ReactNode[] = []
  let list: ReactNode[] = []
  const flush = () => {
    if (list.length) out.push(<ul key={`u${out.length}`}>{list}</ul>)
    list = []
  }
  text.split(/\r?\n/).forEach((line, i) => {
    const t = line.trim()
    if (/^[-*] /.test(t)) return list.push(<li key={i}>{inline(t.slice(2))}</li>)
    flush()
    if (!t) return
    const h = t.match(/^#{1,6}\s+(.*)/)
    out.push(h ? <h5 key={i}>{inline(h[1])}</h5> : <p key={i}>{inline(t)}</p>)
  })
  flush()
  return <div className="notes">{out}</div>
}

// ---------- about & updates dialog ----------

interface DialogProps {
  info: AppInfo | null
  state: UpdateState
  progress: UpdateProgress | null
  onClose: () => void
}

export function AboutDialog({ info, state, progress, onClose }: DialogProps) {
  const [settings, setSettings] = useState<UpdateSettings | null>(null)
  const [error, setError] = useState('')
  useEffect(() => { api.updateSettings().then(setSettings) }, [])

  const save = async (channel: string, auto: boolean) => {
    try {
      setSettings(await api.setUpdateSettings(channel, auto))
      return true
    } catch (e) {
      setError(errText(e))
      return false
    }
  }
  const check = () => {
    setError('')
    api.checkForUpdates().catch(() => { /* shown via state */ })
  }
  const rel: Release | null = state.release
  const checked = state.checkedAt ? new Date(state.checkedAt).toLocaleString() : 'never'

  let status: ReactNode
  switch (state.status) {
    case 'checking': status = <span className="muted">Checking for updates…</span>; break
    case 'uptodate': status = <span style={{ color: 'var(--ok)' }}>✓ You're up to date.</span>; break
    case 'available': status = <span><b>Version {rel?.version}</b> is available{state.skipped ? ' (skipped)' : ''}.</span>; break
    case 'error': status = <span className="err-text">{state.error}</span>; break
    case 'downloading':
    case 'installing': status = <ProgressBar state={state} progress={progress} />; break
    default: status = <span className="muted">Not checked yet.</span>
  }

  return (
    <Modal
      title="About AppMODBUS"
      onClose={onClose}
      width={600}
      footer={
        <>
          {rel && <button className="btn outline" onClick={() => api.openReleasePage()}><IconExternal />Release page</button>}
          <span style={{ flex: 1 }} />
          <button className="btn outline" onClick={onClose}>Close</button>
          <button className="btn outline" disabled={state.status === 'checking' || busy(state)} onClick={check}><IconRefresh />Check now</button>
          {rel && state.status !== 'uptodate' && (
            <button className="btn primary" disabled={!info?.canInstall || busy(state)} onClick={() => api.installUpdate().catch(() => { /* shown via state */ })}>
              <IconDownload />Install {rel.version} &amp; restart
            </button>
          )}
        </>
      }
    >
      <div className="about-head">
        <span className="brand-mark" style={{ width: 44, height: 44, fontSize: 16, borderRadius: 10 }}>MB</span>
        <div>
          <div style={{ fontSize: 16, fontWeight: 700 }}>AppMODBUS</div>
          <div className="muted mono" style={{ fontSize: 12 }}>
            v{info?.version} {info?.commit && `(${info.commit.slice(0, 7)})`} · {info?.os}/{info?.arch}
          </div>
        </div>
      </div>

      <div className="form" style={{ marginTop: 14 }}>
        <label className="lbl">Update channel</label>
        <div className="seg">
          {(['stable', 'beta'] as const).map((c) => (
            <button
              key={c}
              className={settings?.updateChannel === c ? 'on' : ''}
              onClick={async () => { if (settings && await save(c, settings.autoCheckUpdates)) check() }}
            >
              {c === 'stable' ? 'Stable' : 'Beta (pre-releases)'}
            </button>
          ))}
        </div>
        <label className="lbl">Automatic check</label>
        <label className="check">
          <input type="checkbox" checked={settings?.autoCheckUpdates ?? true} onChange={(e) => settings && save(settings.updateChannel, e.target.checked)} />
          Check for updates when AppMODBUS starts
        </label>
        <label className="lbl">Status</label>
        <div>{status}</div>
        <span className="hint">Last checked: {checked} · source: github.com/{info?.repo}</span>
        {info && !info.canInstall && (
          <div className="alert full" style={{ background: 'var(--panel-2)', color: 'var(--muted)' }}>
            This build has no update signing key (development build), so updates can be checked but must be installed manually.
          </div>
        )}
        {error && <div className="alert full">{error}</div>}
      </div>

      {rel && state.status !== 'uptodate' && (
        <div className="section" style={{ marginTop: 14, marginBottom: 0 }}>
          <h4>
            What's new in {rel.name || rel.version}
            {rel.prerelease && <span className="pill beta">Beta</span>}
            <span style={{ flex: 1 }} />
            <span className="muted" style={{ fontWeight: 400 }}>{rel.publishedAt && new Date(rel.publishedAt).toLocaleDateString()} · {fmtBytes(rel.asset.size)}</span>
          </h4>
          <div className="sbody" style={{ maxHeight: 220, overflow: 'auto' }}><Notes text={rel.notes} /></div>
        </div>
      )}
      <p className="muted" style={{ fontSize: 11.5, margin: '12px 0 0' }}>
        Updates are verified with an Ed25519 signature and SHA-256 checksum before they are installed.
      </p>
    </Modal>
  )
}
