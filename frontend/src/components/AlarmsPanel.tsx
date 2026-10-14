import { useEffect, useState } from 'react'
import { api, errText } from '../api'
import { fmtNum } from '../charts/data'
import { ALARM_LEVEL_LABELS, type Alarm, type AlarmEvent, type Poll } from '../types'
import { IconBell, IconCheck, IconSave, IconX } from './Icons'

const SOUND_KEY = 'appmodbus.alarmSound'

export function loadAlarmSound(): boolean {
  try {
    return localStorage.getItem(SOUND_KEY) !== 'off'
  } catch {
    return true
  }
}
export function saveAlarmSound(on: boolean) {
  try {
    localStorage.setItem(SOUND_KEY, on ? 'on' : 'off')
  } catch {
    /* storage unavailable */
  }
}

const time = (ms: number) => (ms ? new Date(ms).toLocaleTimeString() : '')
const dateTime = (ms: number) => new Date(ms).toLocaleString()
const EVENT_LABELS: Record<AlarmEvent['kind'], string> = { active: 'Raised', level: 'Level changed', cleared: 'Returned to normal', ack: 'Acknowledged' }

/** Severity badge: icon + text, never colour alone. */
export function SeverityBadge({ severity }: { severity: 'critical' | 'warning' }) {
  return (
    <span className={`sev ${severity}`}>
      <span className="sev-icon" aria-hidden>{severity === 'critical' ? '!' : '▲'}</span>
      {severity === 'critical' ? 'Critical' : 'Warning'}
    </span>
  )
}

interface Props {
  visible: boolean
  alarms: Alarm[]
  polls: Record<string, Poll>
  sound: boolean
  onSound: (on: boolean) => void
  onClose: () => void
  onError: (m: string) => void
  onInfo: (m: string) => void
}

/** Alarm list with acknowledge, plus the logged history. */
export function AlarmsPanel({ visible, alarms, polls, sound, onSound, onClose, onError, onInfo }: Props) {
  const [tab, setTab] = useState<'list' | 'history'>('list')
  const [days, setDays] = useState(1)
  const [history, setHistory] = useState<AlarmEvent[]>([])
  const run = (fn: () => Promise<void>) => fn().catch((e) => onError(errText(e)))

  useEffect(() => {
    if (!visible || tab !== 'history') return
    const load = () => api.alarmHistory(Date.now() - days * 86_400_000, Date.now(), 2000).then(setHistory).catch((e) => onError(errText(e)))
    load()
    const t = setInterval(load, 3000)
    return () => clearInterval(t)
  }, [visible, tab, days, onError])

  if (!visible) return null
  const unacked = alarms.filter((a) => !a.acked).length
  const current = (a: Alarm) => {
    const cell = polls[a.pollId]?.state.cells.find((c) => c.address === a.address)
    return cell?.hasNum ? fmtNum(cell.num) : '—'
  }
  const exportCSV = () => run(async () => {
    const r = await api.exportAlarmHistory(Date.now() - days * 86_400_000, Date.now())
    if (r.path) onInfo(`Exported ${r.rows} alarm event(s) to ${r.path}`)
  })

  return (
    <div className="traffic alarms">
      <div className="panel-head">
        <h3>Alarms</h3>
        <div className="seg sm-seg">
          <button className={tab === 'list' ? 'on' : ''} onClick={() => setTab('list')}>Current ({alarms.length})</button>
          <button className={tab === 'history' ? 'on' : ''} onClick={() => setTab('history')}>History</button>
        </div>
        {tab === 'list' ? (
          <button className="btn sm" disabled={unacked === 0} onClick={() => run(() => api.ackAlarms([]))}><IconCheck />Acknowledge all</button>
        ) : (
          <>
            <select style={{ width: 130, height: 26 }} value={days} onChange={(e) => setDays(Number(e.target.value))}>
              <option value={1}>Last 24 hours</option>
              <option value={7}>Last 7 days</option>
              <option value={30}>Last 30 days</option>
            </select>
            <button className="btn sm" onClick={exportCSV}><IconSave />Export CSV…</button>
          </>
        )}
        <label className="check" style={{ marginLeft: 8 }}>
          <input type="checkbox" checked={sound} onChange={(e) => { saveAlarmSound(e.target.checked); onSound(e.target.checked) }} /> Sound
        </label>
        <span style={{ flex: 1 }} />
        <button className="btn sm icon" onClick={onClose} title="Hide"><IconX /></button>
      </div>
      <div className="alarm-body">
        {tab === 'list' ? (
          alarms.length === 0 ? (
            <div className="muted alarm-empty"><IconBell /> No alarms. Set limits on a register with right-click → Name / scale this register.</div>
          ) : (
            <table className="tbl alarm-tbl">
              <thead>
                <tr>
                  <th>Severity</th><th>Raised</th><th>Window</th><th>Tag</th><th>Level</th>
                  <th className="num">Value</th><th className="num">Limit</th><th className="num">Now</th><th>State</th><th />
                </tr>
              </thead>
              <tbody>
                {alarms.map((a) => (
                  <tr key={a.id} className={`${a.active ? 'active' : 'cleared'}${a.acked ? ' acked' : ''}`}>
                    <td><SeverityBadge severity={a.severity} /></td>
                    <td className="mono" title={dateTime(a.since)}>{time(a.since)}</td>
                    <td className="ell">{a.window}</td>
                    <td className="ell"><b>{a.name}</b></td>
                    <td>{ALARM_LEVEL_LABELS[a.level]}</td>
                    {a.level === 'comm' ? (
                      <td colSpan={3} className="ell err-text">{a.message}</td>
                    ) : (
                      <>
                        <td className="num">{fmtNum(a.value)} <span className="muted">{a.unit}</span></td>
                        <td className="num">{fmtNum(a.limit)}</td>
                        <td className="num">{current(a)}</td>
                      </>
                    )}
                    <td>
                      {a.active ? 'Active' : `Normal since ${time(a.clearedAt)}`}
                      {a.acked ? <span className="muted"> · acknowledged</span> : <b> · unacknowledged</b>}
                    </td>
                    <td className="acts">
                      {!a.acked && <button className="btn sm outline" onClick={() => run(() => api.ackAlarms([a.id]))}>Acknowledge</button>}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          )
        ) : history.length === 0 ? (
          <div className="muted alarm-empty">No alarm events in this period.</div>
        ) : (
          <table className="tbl alarm-tbl">
            <thead><tr><th>Time</th><th>Event</th><th>Window</th><th>Tag</th><th>Level</th><th className="num">Value</th><th className="num">Limit</th><th>Message</th></tr></thead>
            <tbody>
              {history.map((h, i) => (
                <tr key={`${h.t}-${i}`}>
                  <td className="mono">{dateTime(h.t)}</td>
                  <td>{EVENT_LABELS[h.kind]}</td>
                  <td className="ell">{h.window}</td>
                  <td className="ell">{h.name}</td>
                  <td>{ALARM_LEVEL_LABELS[h.level] ?? h.level}</td>
                  <td className="num">{h.level === 'comm' ? '' : <>{fmtNum(h.value)} <span className="muted">{h.unit}</span></>}</td>
                  <td className="num">{h.level === 'comm' ? '' : fmtNum(h.limit)}</td>
                  <td className="ell muted">{h.message}</td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </div>
    </div>
  )
}

let audio: AudioContext | null = null

/** A short beep: two tones for critical alarms, one for warnings. */
export function beep(critical: boolean) {
  try {
    audio ??= new AudioContext()
    const ctx = audio
    const tones = critical ? [880, 660] : [660]
    tones.forEach((f, i) => {
      const o = ctx.createOscillator()
      const g = ctx.createGain()
      o.frequency.value = f
      o.type = 'sine'
      const t0 = ctx.currentTime + i * 0.22
      g.gain.setValueAtTime(0.0001, t0)
      g.gain.exponentialRampToValueAtTime(0.25, t0 + 0.02)
      g.gain.exponentialRampToValueAtTime(0.0001, t0 + 0.18)
      o.connect(g).connect(ctx.destination)
      o.start(t0)
      o.stop(t0 + 0.2)
    })
  } catch {
    /* no audio */
  }
}
