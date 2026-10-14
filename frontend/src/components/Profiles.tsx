import { useEffect, useMemo, useState } from 'react'
import { api, errText } from '../api'
import type {
  ConnectionInfo, FormatInfo, ImportField, ImportOptions, ImportPreview, ImportResult, Poll, Profile, ProfileFolders,
  ProfileRegister, ProfileSummary,
} from '../types'
import { IconBook, IconCopy, IconPlay, IconPlus, IconSave, IconTrash, IconUpload } from './Icons'
import { Modal } from './Modal'

const SOURCE_LABEL = { bundled: 'Bundled', shared: 'Shared (team)', user: 'My profiles' } as const
const FN_LABEL: Record<number, string> = { 1: '01 Coil', 2: '02 Discrete', 3: '03 Holding', 4: '04 Input' }
const FIELD_LABEL: Record<ImportField, string> = {
  address: 'Address', function: 'Function', name: 'Name', format: 'Data type', gain: 'Gain', offset: 'Offset',
  unit: 'Unit', decimals: 'Decimals', note: 'Note',
}

export const blankProfile = (): Profile => ({
  format: 'appmodbus-profile', version: 1, name: '', manufacturer: '', model: '', description: '', defaultSlaveId: 1,
  registers: [{ address: 0, function: 3, name: '', format: 'unsigned', gain: 1, offset: 0, unit: '', decimals: -1 }],
})

interface Props {
  connections: ConnectionInfo[]
  formats: FormatInfo[]
  activePollId: string
  onApplied: (polls: Poll[]) => void
  onClose: () => void
  onError: (m: string) => void
}

/** Device profile library: browse, edit, import and apply register maps. */
export function ProfilesDialog({ connections, formats, activePollId, onApplied, onClose, onError }: Props) {
  const [list, setList] = useState<ProfileSummary[]>([])
  const [folders, setFolders] = useState<ProfileFolders>({ user: '', shared: '' })
  const [selId, setSelId] = useState('')
  const [draft, setDraft] = useState<Profile | null>(null)
  const [dirty, setDirty] = useState(false)
  const [error, setError] = useState('')
  const [importing, setImporting] = useState<ImportPreview | null>(null)
  const [apply, setApply] = useState({ connectionId: connections[0]?.id ?? '', slaveId: 1, scanRateMs: 1000 })

  const sel = list.find((p) => p.id === selId)
  const readOnly = !draft || sel?.source === 'bundled'

  const refresh = async (keep?: string) => {
    const [l, f] = await Promise.all([api.listProfiles(), api.profileFolders()])
    setList(l)
    setFolders(f)
    const id = keep ?? selId ?? l[0]?.id
    if (id && l.some((p) => p.id === id)) await select(id, l)
    else if (l[0]) await select(l[0].id, l)
  }
  useEffect(() => { refresh() }, [])

  const select = async (id: string, l = list) => {
    setSelId(id)
    setError('')
    setDirty(false)
    const s = l.find((p) => p.id === id)
    if (s?.error) {
      setDraft(null)
      setError(`Cannot read ${s.path}: ${s.error}`)
      return
    }
    try {
      const p = await api.getProfile(id)
      setDraft(p)
      setApply((a) => ({ ...a, slaveId: p.defaultSlaveId || 1 }))
    } catch (e) {
      setDraft(null)
      setError(errText(e))
    }
  }

  const edit = (fn: (p: Profile) => Profile) => {
    if (!draft) return
    setDraft(fn(draft))
    setDirty(true)
  }
  const setReg = (i: number, patch: Partial<ProfileRegister>) =>
    edit((p) => ({ ...p, registers: p.registers.map((r, k) => (k === i ? { ...r, ...patch } : r)) }))

  const save = async (dest: 'user' | 'shared', asNew: boolean) => {
    if (!draft) return
    setError('')
    try {
      const id = await api.saveProfile(draft, dest, asNew ? '' : selId)
      await refresh(id)
    } catch (e) {
      setError(errText(e))
    }
  }
  const startNew = (p: Profile) => {
    setSelId('')
    setDraft(p)
    setDirty(true)
    setError('')
  }
  const remove = async () => {
    if (!sel || sel.source === 'bundled') return
    try {
      await api.deleteProfile(sel.id)
      setSelId('')
      await refresh('')
    } catch (e) {
      setError(errText(e))
    }
  }
  const doApply = async () => {
    if (!sel || dirty) return setError('Save the profile before applying it')
    try {
      const created = await api.applyProfile(sel.id, apply.connectionId, apply.slaveId, apply.scanRateMs)
      onApplied(created)
      onClose()
    } catch (e) {
      setError(errText(e))
    }
  }

  const groups = (['bundled', 'shared', 'user'] as const).map((src) => ({ src, items: list.filter((p) => p.source === src) }))
  const regFormats = formats

  if (importing) {
    return (
      <ImportDialog
        preview={importing}
        hasShared={!!folders.shared}
        onCancel={() => setImporting(null)}
        onSaved={async (id) => { setImporting(null); await refresh(id) }}
      />
    )
  }

  return (
    <Modal
      title="Device profiles"
      onClose={onClose}
      width={1180}
      footer={
        <>
          <span className="muted" style={{ fontSize: 12 }}>
            My profiles: <span className="mono">{folders.user}</span>
            {folders.shared && <> · Shared: <span className="mono">{folders.shared}</span></>}
          </span>
          <span style={{ flex: 1 }} />
          <button className="btn outline" onClick={onClose}>Close</button>
        </>
      }
    >
      <div className="prof-toolbar">
        <button className="btn sm outline" onClick={() => startNew(blankProfile())}><IconPlus />New</button>
        <button className="btn sm outline" onClick={async () => { try { const p = await api.importTable(); if (p) setImporting(p) } catch (e) { setError(errText(e)) } }}>
          <IconUpload />Import CSV / Excel…
        </button>
        <button className="btn sm outline" onClick={async () => { try { const id = await api.importProfileFile(); if (id) await refresh(id) } catch (e) { setError(errText(e)) } }}>
          <IconUpload />Import profile file…
        </button>
        <button
          className="btn sm outline"
          disabled={!activePollId}
          title="Create a profile from the active poll window's formats and tags"
          onClick={async () => { try { startNew(await api.profileFromPoll(activePollId, '')) } catch (e) { setError(errText(e)) } }}
        >
          <IconCopy />From active window
        </button>
        <span style={{ flex: 1 }} />
        {folders.shared ? (
          <button className="btn sm" onClick={async () => { await api.clearSharedProfileFolder(); await refresh() }}>Stop using shared folder</button>
        ) : (
          <button className="btn sm" onClick={async () => { try { if (await api.chooseSharedProfileFolder()) await refresh() } catch (e) { onError(errText(e)) } }}>
            Use a shared folder…
          </button>
        )}
      </div>

      <div className="prof-layout">
        <div className="prof-list">
          {groups.map(({ src, items }) => (src === 'shared' && !folders.shared ? null : (
            <div key={src}>
              <div className="grp">{SOURCE_LABEL[src]}</div>
              {items.length === 0 && <div className="muted prof-empty">{src === 'user' ? 'None yet — import or create one' : 'None'}</div>}
              {items.map((p) => (
                <div key={p.id} className={`conn-item${p.id === selId ? ' sel' : ''}`} onClick={() => select(p.id)}>
                  <IconBook />
                  <div style={{ minWidth: 0 }}>
                    <div className="nm">{p.name}</div>
                    <div className="sm">{p.error ? 'unreadable' : `${p.manufacturer || '—'} · ${p.registers} registers`}</div>
                  </div>
                </div>
              ))}
            </div>
          )))}
        </div>

        <div className="prof-detail">
          {!draft ? (
            <div className="muted" style={{ padding: 20 }}>{error || 'Select a profile.'}</div>
          ) : (
            <>
              <div className="form prof-meta">
                <label className="lbl">Name</label>
                <input type="text" disabled={readOnly && !!selId} value={draft.name} onChange={(e) => edit((p) => ({ ...p, name: e.target.value }))} />
                <label className="lbl">Manufacturer / model</label>
                <div className="inline">
                  <input type="text" disabled={readOnly && !!selId} value={draft.manufacturer} placeholder="Manufacturer" onChange={(e) => edit((p) => ({ ...p, manufacturer: e.target.value }))} />
                  <input type="text" disabled={readOnly && !!selId} value={draft.model} placeholder="Model" onChange={(e) => edit((p) => ({ ...p, model: e.target.value }))} />
                </div>
                <label className="lbl">Description</label>
                <input type="text" disabled={readOnly && !!selId} value={draft.description} onChange={(e) => edit((p) => ({ ...p, description: e.target.value }))} />
              </div>

              <div className="prof-regs">
                <table className="tbl">
                  <thead>
                    <tr><th>Function</th><th>Address</th><th>Name</th><th>Format</th><th>Gain</th><th>Offset</th><th>Unit</th><th>Dec.</th><th /></tr>
                  </thead>
                  <tbody>
                    {draft.registers.map((r, i) => {
                      const ro = readOnly && !!selId
                      return (
                        <tr key={i}>
                          <td>
                            <select disabled={ro} value={r.function} onChange={(e) => setReg(i, { function: Number(e.target.value), format: Number(e.target.value) <= 2 ? 'bit' : r.format === 'bit' ? 'unsigned' : r.format })}>
                              {[3, 4, 1, 2].map((f) => <option key={f} value={f}>{FN_LABEL[f]}</option>)}
                            </select>
                          </td>
                          <td><input type="number" disabled={ro} min={0} max={65535} value={r.address} onChange={(e) => setReg(i, { address: Number(e.target.value) })} /></td>
                          <td><input type="text" disabled={ro} value={r.name} onChange={(e) => setReg(i, { name: e.target.value })} /></td>
                          <td>
                            {r.function <= 2 ? <span className="muted">bit</span> : (
                              <select disabled={ro} value={r.format} onChange={(e) => setReg(i, { format: e.target.value })}>
                                {regFormats.map((f) => <option key={f.id} value={f.id}>{f.label}</option>)}
                              </select>
                            )}
                          </td>
                          <td><input type="text" className="mono" disabled={ro || r.function <= 2} value={r.gain} onChange={(e) => setReg(i, { gain: Number(e.target.value) || 0 })} /></td>
                          <td><input type="text" className="mono" disabled={ro || r.function <= 2} value={r.offset} onChange={(e) => setReg(i, { offset: Number(e.target.value) || 0 })} /></td>
                          <td><input type="text" disabled={ro} value={r.unit} onChange={(e) => setReg(i, { unit: e.target.value })} /></td>
                          <td>
                            <select disabled={ro || r.function <= 2} value={r.decimals} onChange={(e) => setReg(i, { decimals: Number(e.target.value) })}>
                              <option value={-1}>auto</option>
                              {[0, 1, 2, 3, 4, 5, 6].map((d) => <option key={d} value={d}>{d}</option>)}
                            </select>
                          </td>
                          <td>
                            {!ro && (
                              <button className="btn sm icon" title="Remove" onClick={() => edit((p) => ({ ...p, registers: p.registers.filter((_, k) => k !== i) }))}><IconTrash /></button>
                            )}
                          </td>
                        </tr>
                      )
                    })}
                  </tbody>
                </table>
                {!(readOnly && !!selId) && (
                  <button
                    className="btn sm outline"
                    style={{ marginTop: 8 }}
                    onClick={() => edit((p) => {
                      const last = p.registers[p.registers.length - 1]
                      const next = last ? last.address + (formats.find((f) => f.id === last.format)?.registers ?? 1) : 0
                      return { ...p, registers: [...p.registers, { address: next, function: last?.function ?? 3, name: '', format: last?.format ?? 'unsigned', gain: 1, offset: 0, unit: '', decimals: -1 }] }
                    })}
                  >
                    <IconPlus />Add register
                  </button>
                )}
              </div>

              <div className="prof-actions">
                <div className="apply-box">
                  <b>Apply to a device</b>
                  <select value={apply.connectionId} onChange={(e) => setApply((a) => ({ ...a, connectionId: e.target.value }))}>
                    {connections.length === 0 && <option value="">— create a connection first —</option>}
                    {connections.map((c) => <option key={c.id} value={c.id}>{c.name}</option>)}
                  </select>
                  <label className="muted">Slave ID</label>
                  <input type="number" min={0} max={255} style={{ width: 70 }} value={apply.slaveId} onChange={(e) => setApply((a) => ({ ...a, slaveId: Number(e.target.value) }))} />
                  <label className="muted">Scan</label>
                  <input type="number" min={10} style={{ width: 80 }} value={apply.scanRateMs} onChange={(e) => setApply((a) => ({ ...a, scanRateMs: Number(e.target.value) }))} />
                  <span className="muted">ms</span>
                  <button className="btn primary sm" disabled={!sel || !apply.connectionId} onClick={doApply}><IconPlay />Create windows</button>
                </div>
                <span style={{ flex: 1 }} />
                {sel && sel.source !== 'bundled' && <button className="btn sm danger" onClick={remove}><IconTrash />Delete</button>}
                {sel && <button className="btn sm outline" onClick={async () => { try { await api.exportProfile(sel.id) } catch (e) { setError(errText(e)) } }}>Export…</button>}
                {(readOnly && !!selId) ? (
                  <button className="btn sm outline" onClick={() => save('user', true)}><IconCopy />Copy to My profiles</button>
                ) : (
                  <>
                    {folders.shared && <button className="btn sm outline" disabled={!dirty && !!selId} onClick={() => save('shared', sel?.source !== 'shared')}><IconSave />Save to shared</button>}
                    <button className="btn sm primary" disabled={!dirty} onClick={() => save(sel?.source === 'shared' ? 'shared' : 'user', !selId)}><IconSave />Save</button>
                  </>
                )}
              </div>
              {error && <div className="alert" style={{ marginTop: 8 }}>{error}</div>}
            </>
          )}
        </div>
      </div>
    </Modal>
  )
}

// ---------- import from CSV / Excel ----------

interface ImportProps {
  preview: ImportPreview
  hasShared: boolean
  onCancel: () => void
  onSaved: (id: string) => void
}

export function ImportDialog({ preview, hasShared, onCancel, onSaved }: ImportProps) {
  const [opt, setOpt] = useState<ImportOptions>(preview.options)
  const [result, setResult] = useState<ImportResult | null>(null)
  const [meta, setMeta] = useState({ name: preview.fileName.replace(/\.[^.]+$/, ''), manufacturer: '', model: '' })
  const [error, setError] = useState('')

  useEffect(() => {
    api.buildImport(preview.path, opt).then(setResult).catch((e) => setError(errText(e)))
  }, [opt, preview.path])

  const width = useMemo(() => Math.max(0, ...preview.rows.slice(0, 50).map((r) => r.length)), [preview.rows])
  const colField = (c: number) => (Object.entries(opt.mapping) as [ImportField, number][]).find(([, v]) => v === c)?.[0] ?? ''
  const mapCol = (c: number, f: ImportField | '') => setOpt((o) => {
    const m = { ...o.mapping }
    for (const [k, v] of Object.entries(m)) if (v === c) delete m[k as ImportField]
    if (f) m[f] = c
    return { ...o, mapping: m }
  })

  const save = async (dest: 'user' | 'shared') => {
    if (!result) return
    try {
      const id = await api.saveProfile({ ...result.profile, ...meta, format: 'appmodbus-profile', version: 1, description: `Imported from ${preview.fileName}`, defaultSlaveId: 1 }, dest, '')
      onSaved(id)
    } catch (e) {
      setError(errText(e))
    }
  }
  const regs = result?.profile.registers ?? []

  return (
    <Modal
      title={`Import register map — ${preview.fileName}`}
      onClose={onCancel}
      width={1180}
      footer={
        <>
          <span className="muted">{regs.length} registers{result?.warnings.length ? ` · ${result.warnings.length} warnings` : ''}</span>
          <span style={{ flex: 1 }} />
          <button className="btn outline" onClick={onCancel}>Back</button>
          {hasShared && <button className="btn outline" disabled={!regs.length} onClick={() => save('shared')}>Save to shared</button>}
          <button className="btn primary" disabled={!regs.length} onClick={() => save('user')}>Save to My profiles</button>
        </>
      }
    >
      <div className="import-opts">
        <label>Header row <input type="number" min={1} value={opt.headerRow + 1} onChange={(e) => setOpt((o) => ({ ...o, headerRow: Math.max(0, Number(e.target.value) - 1) }))} /></label>
        <label>Addresses
          <select value={opt.addressMode} onChange={(e) => setOpt((o) => ({ ...o, addressMode: e.target.value as ImportOptions['addressMode'] }))}>
            <option value="zero">Protocol address (0-based)</option>
            <option value="one">Register number (1-based)</option>
            <option value="modicon">Modicon 3xxxx / 4xxxx (sets function)</option>
          </select>
        </label>
        <label>Default function
          <select value={opt.defaultFunction} onChange={(e) => setOpt((o) => ({ ...o, defaultFunction: Number(e.target.value) }))}>
            {[3, 4, 1, 2].map((f) => <option key={f} value={f}>{FN_LABEL[f]}</option>)}
          </select>
        </label>
        <label>32/64-bit byte order
          <select value={opt.byteOrder} onChange={(e) => setOpt((o) => ({ ...o, byteOrder: e.target.value as ImportOptions['byteOrder'] }))}>
            <option value="abcd">AB CD (big endian)</option><option value="cdab">CD AB (word swap)</option>
            <option value="badc">BA DC (byte swap)</option><option value="dcba">DC BA (little endian)</option>
          </select>
        </label>
        <label>Name <input type="text" value={meta.name} onChange={(e) => setMeta((m) => ({ ...m, name: e.target.value }))} /></label>
        <label>Manufacturer <input type="text" value={meta.manufacturer} onChange={(e) => setMeta((m) => ({ ...m, manufacturer: e.target.value }))} /></label>
        <label>Model <input type="text" value={meta.model} onChange={(e) => setMeta((m) => ({ ...m, model: e.target.value }))} /></label>
      </div>

      <div className="import-split">
        <div className="import-sheet">
          <div className="muted" style={{ fontSize: 12, margin: '0 0 6px' }}>Spreadsheet — choose what each column contains</div>
          <div className="sheet-scroll">
            <table className="tbl sheet">
              <thead>
                <tr>
                  <th>#</th>
                  {Array.from({ length: width }, (_, c) => (
                    <th key={c}>
                      <select value={colField(c)} onChange={(e) => mapCol(c, e.target.value as ImportField | '')} className={colField(c) ? 'mapped' : ''}>
                        <option value="">— ignore —</option>
                        {preview.fields.map((f) => <option key={f} value={f}>{FIELD_LABEL[f]}</option>)}
                      </select>
                    </th>
                  ))}
                </tr>
              </thead>
              <tbody>
                {preview.rows.slice(0, 80).map((row, r) => (
                  <tr key={r} className={r === opt.headerRow ? 'hdr' : r < opt.headerRow ? 'skip' : ''}>
                    <td className="muted">{r + 1}</td>
                    {Array.from({ length: width }, (_, c) => <td key={c} className={colField(c) ? 'mapped' : ''}>{row[c] ?? ''}</td>)}
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </div>
        <div className="import-result">
          <div className="muted" style={{ fontSize: 12, margin: '0 0 6px' }}>Result</div>
          <div className="sheet-scroll">
            <table className="tbl">
              <thead><tr><th>FC</th><th>Addr</th><th>Name</th><th>Format</th><th>Gain</th><th>Unit</th></tr></thead>
              <tbody>
                {regs.slice(0, 300).map((r, i) => (
                  <tr key={i}>
                    <td className="mono">{String(r.function).padStart(2, '0')}</td>
                    <td className="mono">{r.address}</td>
                    <td>{r.name}</td>
                    <td className="mono">{r.format}</td>
                    <td className="mono">{r.gain !== 1 ? r.gain : ''}</td>
                    <td>{r.unit}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
          {!!result?.warnings.length && (
            <details className="warn-list">
              <summary>{result.warnings.length} rows need attention</summary>
              {result.warnings.slice(0, 50).map((w, i) => <div key={i}>{w}</div>)}
            </details>
          )}
          {error && <div className="alert">{error}</div>}
        </div>
      </div>
    </Modal>
  )
}
