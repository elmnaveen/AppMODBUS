// Typed wrapper around the Wails-generated Go bindings.
import * as Go from '../wailsjs/go/main/App'
import { EventsOn } from '../wailsjs/runtime/runtime'
import type {
  Layout, LoadResult, WorkspaceState,
  AppInfo, UpdateProgress, UpdateSettings, UpdateState,
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

  simConfig: (): Promise<SimConfig> => g.SimConfig(),

  restoreSession: (): Promise<LoadResult> => g.RestoreSession(),
  workspaceState: (): Promise<WorkspaceState> => g.WorkspaceState(),
  newWorkspace: (): Promise<LoadResult> => g.NewWorkspace(),
  openWorkspace: (): Promise<LoadResult | null> => g.OpenWorkspace(),
  openWorkspacePath: (path: string): Promise<LoadResult | null> => g.OpenWorkspacePath(path),
  saveWorkspace: (): Promise<string> => g.SaveWorkspace(),
  saveWorkspaceAs: (): Promise<string> => g.SaveWorkspaceAs(),
  setLayout: (l: Layout): Promise<void> => g.SetLayout(l),
  setCharts: (charts: unknown): Promise<void> => g.SetCharts(charts),
  remapSerialPort: (connectionId: string, port: string): Promise<void> => g.RemapSerialPort(connectionId, port),

  appInfo: (): Promise<AppInfo> => g.AppInfo(),
  updateSettings: (): Promise<UpdateSettings> => g.UpdateSettings(),
  setUpdateSettings: (channel: string, autoCheck: boolean): Promise<UpdateSettings> => g.SetUpdateSettings(channel, autoCheck),
  updateState: (): Promise<UpdateState> => g.GetUpdateState(),
  checkForUpdates: (): Promise<UpdateState> => g.CheckForUpdates(),
  skipVersion: (v: string): Promise<void> => g.SkipVersion(v),
  openReleasePage: (): Promise<void> => g.OpenReleasePage(),
  installUpdate: (): Promise<void> => g.InstallUpdate(),
}

export const events = {
  onPoll: (cb: (p: Poll) => void) => EventsOn('poll:update', cb),
  onConnections: (cb: (c: ConnectionInfo[]) => void) => EventsOn('connections', cb),
  onTraffic: (cb: (t: TrafficEntry[]) => void) => EventsOn('traffic', cb),
  onWorkspace: (cb: (s: WorkspaceState) => void) => EventsOn('workspace:state', cb),
  onUpdateState: (cb: (s: UpdateState) => void) => EventsOn('update:state', cb),
  onUpdateProgress: (cb: (p: UpdateProgress) => void) => EventsOn('update:progress', cb),
}

/** Turns a rejected binding promise into a display string. */
export const errText = (e: unknown) => (typeof e === 'string' ? e : e instanceof Error ? e.message : String(e))
