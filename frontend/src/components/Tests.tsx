import { useEffect, useState } from 'react'
import { api, errText, events } from '../api'
import type { CheckOp, ConnectionInfo, FormatInfo, Sequence, SequenceRun, SeqStep, StepKind, StepResult } from '../types'
import { IconCopy, IconPlay, IconPlus, IconSave, IconStop, IconTrash } from './Icons'
import { Modal } from './Modal'

export const newSequenceId = () => `seq-${Date.now().toString(36)}${Math.random().toString(36).slice(2, 6)}`

const blankStep = (kind: StepKind): SeqStep => ({
  kind, label: '', slaveId: 0, function: kind === 'write' ? 6 : 3, address: 0, format: 'signed', gain: 1, offset: 0,
  check: kind === 'read' ? 'range' : '', value: 0, min: 0, max: 0, tolerance: 0, values: '', waitMs: 1000, text: '',
})

export const blankSequence = (connectionId: string, n: number): Sequence => ({
  id: newSequenceId(), name: `Test ${n}`, connectionId, slaveId: 1, repeat: 1, stopOnFail: false, stepDelayMs: 0,
  steps: [blankStep('read')],
})

/** Accepts whatever the workspace stored. */
export function parseSequences(raw: unknown): Sequence[] {
  if (!Array.isArray(raw)) return []
  return raw
    .filter((s): s is Sequence => !!s && typeof s.id === 'string' && Array.isArray(s.steps))
    .map((s) => ({ ...blankSequence('', 1), ...s, steps: s.steps.map((st) => ({ ...blankStep(st.kind ?? 'read'), ...st })) }))
}

const CHECKS: { id: CheckOp; label: string }[] = [
  { id: 'range', label: 'between' },
  { id: 'eq', label: '=' },
  { id: 'ne', label: '≠' },
  { id: 'lt', label: '<' },
  { id: 'le', label: '≤' },
  { id: 'gt', label: '>' },
  { id: 'ge', label: '≥' },
  { id: '', label: 'record only' },
]
const READ_FNS = [1, 2, 3, 4]
const WRITE_FNS = [5, 6, 15, 16]
const FN_LABEL: Record<number, string> = { 1: '01 Coil', 2: '02 Discrete', 3: '03 Holding', 4: '04 Input', 5: '05 Coil', 6: '06 Register', 15: '15 Coils', 16: '16 Registers' }
const KIND_LABEL: Record<StepKind, string> = { read: 'Read & check', write: 'Write', wait: 'Wait', prompt: 'Operator' }

const num = (s: string) => (s.trim() === '' ? 0 : Number(s))

interface Props {
  sequences: Sequence[]
  connections: ConnectionInfo[]
  formats: FormatInfo[]
  onChange: (s: Sequence[]) => void
  onClose: () => void
  onInfo: (m: string) => void
}

/** Test sequences: edit, run against a device, see and export the report. */
export function TestsDialog({ sequences, connections, formats, onChange, onClose, onInfo }: Props) {
  const [sel, setSel] = useState(sequences[0]?.id ?? '')
  const [run, setRun] = useState<SequenceRun | null>(null)
  const [error, setError] = useState('')
  const seq = sequences.find((s) => s.id === sel)

  useEffect(() => {
    const keep = (r: SequenceRun) => setRun((cur) => (cur && cur.seq > r.seq ? cur : r))
    api.sequenceState().then(keep)
    return events.onSequence(keep)
  }, [])
  useEffect(() => { if (!seq && sequences[0]) setSel(sequences[0].id) }, [seq, sequences])

  const update = (s: Sequence) => onChange(sequences.map((x) => (x.id === s.id ? s : x)))
  const add = () => {
    const s = blankSequence(connections[0]?.id ?? '', sequences.length + 1)
    onChange([...sequences, s])
    setSel(s.id)
  }
  const duplicate = () => {
    if (!seq) return
    const s = { ...structuredClone(seq), id: newSequenceId(), name: `${seq.name} (copy)` }
    onChange([...sequences, s])
    setSel(s.id)
  }
  const remove = () => {
    if (!seq) return
    onChange(sequences.filter((s) => s.id !== seq.id))
  }
  const start = async () => {
    if (!seq) return
    setError('')
    try {
      await api.runSequence(seq)
    } catch (e) {
      setError(errText(e))
    }
  }
  const exportReport = async () => {
    try {
      const p = await api.exportSequenceReport()
      if (p) onInfo(`Saved report ${p}`)
    } catch (e) {
      setError(errText(e))
    }
  }

  const running = !!run?.running
  const mine = run && seq && run.sequenceId === seq.id ? run : null
  // Latest result of each step (from the current or last run).
  const latest = new Map<number, StepResult>()
  mine?.results.forEach((r) => latest.set(r.index, r)) // results arrive in order: the last one wins

  return (
    <Modal title="Test sequences" onClose={onClose} width={1180}>
      <div className="conn-layout tests-layout">
        <div>
          <div className="conn-list">
            {sequences.map((s) => (
              <div key={s.id} className={`conn-item${s.id === sel ? ' sel' : ''}`} onClick={() => setSel(s.id)}>
                <IconPlay />
                <div style={{ minWidth: 0 }}>
                  <div className="nm ell">{s.name}</div>
                  <div className="sm">{s.steps.length} steps{s.repeat > 1 ? ` × ${s.repeat}` : ''}</div>
                </div>
                {run?.sequenceId === s.id && run.verdict && !run.running && <span className={`verdict-dot ${run.verdict}`} title={run.verdict} />}
                {run?.sequenceId === s.id && run.running && <span className="scan-dot" />}
              </div>
            ))}
            {sequences.length === 0 && <div className="conn-item muted">No sequences yet</div>}
          </div>
          <div className="row" style={{ marginTop: 8, gap: 6, display: 'flex' }}>
            <button className="btn outline sm" onClick={add}><IconPlus />New</button>
            <button className="btn outline sm" disabled={!seq} onClick={duplicate}><IconCopy />Copy</button>
            <button className="btn outline sm" disabled={!seq || running} onClick={remove}><IconTrash />Delete</button>
          </div>
        </div>
        {!seq ? (
          <div className="muted tests-empty">
            <p>A test sequence reads, writes, waits and asks the operator, then reports pass or fail for each check. Use it for production tests of meters and controllers.</p>
            <button className="btn primary" disabled={connections.length === 0} onClick={add}><IconPlus />New sequence</button>
            {connections.length === 0 && <p>Add a connection first.</p>}
          </div>
        ) : (
          <div className="diag-stack">
            <div className="diag-row">
              <label className="diag-field"><span>Name</span><input type="text" value={seq.name} style={{ width: 200 }} onChange={(e) => update({ ...seq, name: e.target.value })} /></label>
              <label className="diag-field"><span>Connection</span>
                <select value={seq.connectionId} onChange={(e) => update({ ...seq, connectionId: e.target.value })}>
                  {!connections.some((c) => c.id === seq.connectionId) && <option value="">Choose…</option>}
                  {connections.map((c) => <option key={c.id} value={c.id}>{c.name} · {c.summary}</option>)}
                </select>
              </label>
              <label className="diag-field"><span>Slave ID</span><input type="number" min={0} max={255} style={{ width: 64 }} value={seq.slaveId} onChange={(e) => update({ ...seq, slaveId: e.target.valueAsNumber || 0 })} /></label>
              <label className="diag-field"><span>Repeat</span><input type="number" min={1} max={10000} style={{ width: 70 }} value={seq.repeat} onChange={(e) => update({ ...seq, repeat: Math.max(1, e.target.valueAsNumber || 1) })} /></label>
              <label className="diag-field"><span>Pause between steps</span>
                <span className="inline"><input type="number" min={0} style={{ width: 70 }} value={seq.stepDelayMs} onChange={(e) => update({ ...seq, stepDelayMs: Math.max(0, e.target.valueAsNumber || 0) })} /> <span className="muted">ms</span></span>
              </label>
              <label className="check" style={{ alignSelf: 'center', marginTop: 16 }}>
                <input type="checkbox" checked={seq.stopOnFail} onChange={(e) => update({ ...seq, stopOnFail: e.target.checked })} />Stop at first failure
              </label>
            </div>

            <div className="steps-wrap">
              <table className="tbl steps">
                <thead>
                  <tr><th>#</th><th>Step</th><th>Settings</th><th>Check</th><th>Result</th></tr>
                </thead>
                <tbody>
                  {seq.steps.map((st, i) => (
                    <StepRow
                      key={i}
                      i={i}
                      step={st}
                      count={seq.steps.length}
                      formats={formats}
                      result={latest.get(i)}
                      current={running && mine?.step === i}
                      onChange={(s) => update({ ...seq, steps: seq.steps.map((x, j) => (j === i ? s : x)) })}
                      onMove={(d) => {
                        const steps = [...seq.steps]
                        const [x] = steps.splice(i, 1)
                        steps.splice(i + d, 0, x)
                        update({ ...seq, steps })
                      }}
                      onRemove={() => update({ ...seq, steps: seq.steps.filter((_, j) => j !== i) })}
                    />
                  ))}
                </tbody>
              </table>
            </div>
            <div className="diag-row">
              {(Object.keys(KIND_LABEL) as StepKind[]).map((k) => (
                <button key={k} className="btn outline sm" disabled={running} onClick={() => update({ ...seq, steps: [...seq.steps, blankStep(k)] })}>
                  <IconPlus />{KIND_LABEL[k]}
                </button>
              ))}
              <span style={{ flex: 1 }} />
              {mine && mine.started > 0 && <button className="btn outline" onClick={exportReport}><IconSave />Save report…</button>}
              {running ? (
                <button className="btn" onClick={() => api.stopSequence()}><IconStop />Stop</button>
              ) : (
                <button className="btn primary" disabled={!seq.connectionId || seq.steps.length === 0 || (!!run?.running)} onClick={start}><IconPlay />Run</button>
              )}
            </div>
            {error && <div className="alert">{error}</div>}
            {mine && <RunSummary run={mine} />}
            {run?.running && run.sequenceId !== seq.id && <div className="muted">“{run.name}” is running.</div>}
          </div>
        )}
      </div>
      {run?.prompt && (
        <div className="prompt-overlay">
          <div className="prompt-box">
            <div className="prompt-h">Operator step</div>
            <p>{run.prompt}</p>
            <div className="row" style={{ display: 'flex', gap: 8, justifyContent: 'flex-end' }}>
              <button className="btn outline danger" onClick={() => api.answerPrompt(false)}>Fail</button>
              <button className="btn primary" autoFocus onClick={() => api.answerPrompt(true)}>Pass / continue</button>
            </div>
          </div>
        </div>
      )}
    </Modal>
  )
}

function RunSummary({ run }: { run: SequenceRun }) {
  const secs = ((run.finished || Date.now()) - run.started) / 1000
  const label = run.running ? 'Running' : run.verdict === 'pass' ? 'PASS' : run.verdict === 'fail' ? 'FAIL' : run.verdict === 'stopped' ? 'Stopped' : 'Error'
  return (
    <div className={`run-summary ${run.running ? 'running' : run.verdict}`}>
      <span className="verdict">{run.verdict === 'pass' && !run.running ? '✓ ' : run.verdict === 'fail' && !run.running ? '✗ ' : ''}{label}</span>
      <span>{run.passed} passed · {run.failed} failed{run.errors ? ` · ${run.errors} errors` : ''}</span>
      {run.runs > 1 && <span>Run {run.run} of {run.runs}</span>}
      <span className="muted">{secs.toFixed(1)} s · {run.connection}</span>
      {run.error && <span className="err-text">{run.error}</span>}
    </div>
  )
}

const STATUS: Record<StepResult['status'], { text: string; cls: string; icon: string }> = {
  pass: { text: 'Pass', cls: 'pass', icon: '✓' },
  fail: { text: 'Fail', cls: 'fail', icon: '✗' },
  error: { text: 'Error', cls: 'fail', icon: '!' },
  done: { text: 'Done', cls: 'done', icon: '•' },
  skipped: { text: 'Skipped', cls: 'done', icon: '–' },
}

function StepRow({ i, step: st, count, formats, result, current, onChange, onMove, onRemove }: {
  i: number; step: SeqStep; count: number; formats: FormatInfo[]; result?: StepResult; current: boolean
  onChange: (s: SeqStep) => void; onMove: (d: number) => void; onRemove: () => void
}) {
  const set = <K extends keyof SeqStep>(k: K, v: SeqStep[K]) => onChange({ ...st, [k]: v })
  const bits = st.function === 1 || st.function === 2 || st.function === 5 || st.function === 15
  const fmtSel = (
    <select value={st.format} onChange={(e) => set('format', e.target.value)} title="Value format">
      {formats.map((f) => <option key={f.id} value={f.id}>{f.label}</option>)}
    </select>
  )
  const s = result ? STATUS[result.status] : null
  return (
    <tr className={current ? 'current' : ''}>
      <td className="mono muted">{i + 1}</td>
      <td>
        <div className="step-kind">{KIND_LABEL[st.kind]}</div>
        <input type="text" className="step-label" placeholder="Label" value={st.label} onChange={(e) => set('label', e.target.value)} />
        <div className="step-acts">
          <button className="btn sm icon" disabled={i === 0} onClick={() => onMove(-1)} title="Move up">▲</button>
          <button className="btn sm icon" disabled={i === count - 1} onClick={() => onMove(1)} title="Move down">▼</button>
          <button className="btn sm icon" onClick={onRemove} title="Remove step"><IconTrash /></button>
        </div>
      </td>
      <td>
        <div className="step-fields">
          {(st.kind === 'read' || st.kind === 'write') && (
            <>
              <select value={st.function} onChange={(e) => set('function', Number(e.target.value))}>
                {(st.kind === 'read' ? READ_FNS : WRITE_FNS).map((f) => <option key={f} value={f}>{FN_LABEL[f]}</option>)}
              </select>
              <label>addr<input type="number" min={0} max={65535} value={st.address} onChange={(e) => set('address', e.target.valueAsNumber || 0)} /></label>
              <label title="0 = the sequence's slave">ID<input type="number" min={0} max={255} value={st.slaveId} onChange={(e) => set('slaveId', e.target.valueAsNumber || 0)} /></label>
              {!bits && fmtSel}
            </>
          )}
          {st.kind === 'read' && !bits && (
            <>
              <label>×<input type="text" className="mono" value={String(st.gain)} onChange={(e) => set('gain', num(e.target.value))} /></label>
              <label>+<input type="text" className="mono" value={String(st.offset)} onChange={(e) => set('offset', num(e.target.value))} /></label>
            </>
          )}
          {st.kind === 'write' && (
            <label className="grow">value{st.function === 15 || st.function === 16 ? 's' : ''}
              <input type="text" className="mono" placeholder={bits ? 'on / off' : st.function === 16 ? '100, 200' : '1234'} value={st.values} onChange={(e) => set('values', e.target.value)} />
            </label>
          )}
          {st.kind === 'wait' && <label>ms<input type="number" min={0} value={st.waitMs} onChange={(e) => set('waitMs', e.target.valueAsNumber || 0)} /></label>}
          {st.kind === 'prompt' && (
            <input type="text" className="grow" placeholder="Instruction, e.g. Switch the load on and confirm the LED is green" value={st.text} onChange={(e) => set('text', e.target.value)} />
          )}
        </div>
      </td>
      <td>
        {st.kind === 'read' ? (
          <div className="step-fields">
            <select value={st.check} onChange={(e) => set('check', e.target.value as CheckOp)}>
              {CHECKS.map((c) => <option key={c.id} value={c.id}>{c.label}</option>)}
            </select>
            {st.check === 'range' && (
              <>
                <input type="text" className="mono" value={String(st.min)} onChange={(e) => set('min', num(e.target.value))} />
                <span className="muted">and</span>
                <input type="text" className="mono" value={String(st.max)} onChange={(e) => set('max', num(e.target.value))} />
              </>
            )}
            {st.check !== 'range' && st.check !== '' && <input type="text" className="mono" value={String(st.value)} onChange={(e) => set('value', num(e.target.value))} />}
            {(st.check === 'eq' || st.check === 'ne') && <label>±<input type="text" className="mono" value={String(st.tolerance)} onChange={(e) => set('tolerance', num(e.target.value))} /></label>}
          </div>
        ) : st.kind === 'prompt' ? <span className="muted">operator decides</span> : <span className="muted">—</span>}
      </td>
      <td className="step-result">
        {current && !result && <span className="muted">running…</span>}
        {s && (
          <span className={`res ${s.cls}`} title={result?.message}>
            <span className="res-icon">{s.icon}</span>{s.text}
            {result?.value && <span className="mono res-val">{result.value}</span>}
          </span>
        )}
        {result?.message && result.status !== 'pass' && <div className="res-msg" title={result.message}>{result.message}</div>}
      </td>
    </tr>
  )
}
