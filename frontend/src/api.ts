// Typed wrapper around the Wails-generated Go bindings.
import * as Go from '../wailsjs/go/main/App'
import { EventsOn } from '../wailsjs/runtime/runtime'
import type {
  ConnectionConfig, ConnectionInfo, FormatInfo, Poll, PollConfig, SimConfig,
  SimStatus, SimTable, TrafficEntry, WriteRequest,
} from './types'

// The generated signatures use classes from models.ts; our interfaces are
// structurally identical, so cast through `any` at this one boundary.
const g = Go as any

export const api = {
  formats: (): Promise<FormatInfo[]> => g.Formats(),
  serialPorts: (): Promise<string[]> => g.SerialPorts(),

  connections: (): Promise<ConnectionInfo[]> => g.Connections(),
  saveConnection: (c: ConnectionConfig): Promise<ConnectionInfo> => g.SaveConnection(c),
  deleteConnection: (id: string): Promise<void> => g.DeleteConnection(id),
  connect: (id: string): Promise<void> => g.Connect(id),
  disconnect: (id: string): Promise<void> => g.Disconnect(id),

  polls: (): Promise<Poll[]> => g.Polls(),
  createPoll: (c: PollConfig): Promise<Poll> => g.CreatePoll(c),
  updatePoll: (c: PollConfig): Promise<Poll> => g.UpdatePoll(c),
  deletePoll: (id: string): Promise<void> => g.DeletePoll(id),
  startPoll: (id: string): Promise<void> => g.StartPoll(id),
  stopPoll: (id: string): Promise<void> => g.StopPoll(id),
  startAll: (): Promise<void> => g.StartAll(),
  stopAll: (): Promise<void> => g.StopAll(),
  readOnce: (id: string): Promise<void> => g.ReadOnce(id),
  setCellFormat: (id: string, address: number, format: string): Promise<void> =>
    g.SetCellFormat(id, address, format),

  write: (w: WriteRequest): Promise<void> => g.Write(w),
  writeCell: (id: string, address: number, value: string, forceMultiple: boolean): Promise<void> =>
    g.WriteCell(id, address, value, forceMultiple),
  parseValue: (format: string, value: string): Promise<number[]> => g.ParseValue(format, value),

  setTrafficPaused: (p: boolean): Promise<void> => g.SetTrafficPaused(p),
  saveTrafficLog: (text: string): Promise<string> => g.SaveTrafficLog(text),

  simStart: (c: SimConfig): Promise<void> => g.SimStart(c),
  simStop: (): Promise<void> => g.SimStop(),
  simStatus: (): Promise<SimStatus> => g.SimStatus(),
  simRead: (t: SimTable, address: number, qty: number): Promise<number[]> => g.SimRead(t, address, qty),
  simWrite: (t: SimTable, address: number, values: number[]): Promise<void> => g.SimWrite(t, address, values),
}

export const events = {
  onPoll: (cb: (p: Poll) => void) => EventsOn('poll:update', cb),
  onConnections: (cb: (c: ConnectionInfo[]) => void) => EventsOn('connections', cb),
  onTraffic: (cb: (t: TrafficEntry[]) => void) => EventsOn('traffic', cb),
}

/** Turns a rejected binding promise into a display string. */
export const errText = (e: unknown) => (typeof e === 'string' ? e : e instanceof Error ? e.message : String(e))
