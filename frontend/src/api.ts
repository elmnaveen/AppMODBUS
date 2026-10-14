// Typed wrapper around the Wails-generated Go bindings.
import * as Go from '../wailsjs/go/main/App'
import { EventsOn } from '../wailsjs/runtime/runtime'
import type {
  Alarm, AlarmEvent,
  ConnectionStats, CounterValue, DeviceID, EchoResult, IDScanRequest, MapRange, MapScanRequest, NetworkScanRequest, ScanState,
  DataSettings, Sample, SeriesInfo,
  ImportOptions, ImportPreview, ImportResult, Profile, ProfileFolders, ProfileSummary, Tag,
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
  readWrite: (r: { connectionId: string; slaveId: number; readAddress: number; readQuantity: number; writeAddress: number; values: number[] }): Promise<number[]> =>
    g.ReadWrite(r),
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

  setTag: (pollId: string, address: number, tag: Tag | null): Promise<void> => g.SetTag(pollId, address, tag),
  listProfiles: (): Promise<ProfileSummary[]> => g.ListProfiles(),
  getProfile: (id: string): Promise<Profile> => g.GetProfile(id),
  saveProfile: (p: Profile, dest: 'user' | 'shared', id: string): Promise<string> => g.SaveProfile(p, dest, id),
  deleteProfile: (id: string): Promise<void> => g.DeleteProfile(id),
  profileFolders: (): Promise<ProfileFolders> => g.ProfileFolders(),
  chooseSharedProfileFolder: (): Promise<string> => g.ChooseSharedProfileFolder(),
  clearSharedProfileFolder: (): Promise<void> => g.ClearSharedProfileFolder(),
  applyProfile: (id: string, connectionId: string, slaveId: number, scanRateMs: number): Promise<Poll[]> =>
    g.ApplyProfile(id, connectionId, slaveId, scanRateMs),
  profileFromPoll: (pollId: string, name: string): Promise<Profile> => g.ProfileFromPoll(pollId, name),
  importTable: (): Promise<ImportPreview | null> => g.ImportTable(),
  previewTable: (path: string): Promise<ImportPreview> => g.PreviewTable(path),
  buildImport: (path: string, o: ImportOptions): Promise<ImportResult> => g.BuildImport(path, o),
  importProfileFile: (): Promise<string> => g.ImportProfileFile(),
  exportProfile: (id: string): Promise<string> => g.ExportProfile(id),

  setPollRecording: (pollId: string, on: boolean): Promise<void> => g.SetPollRecording(pollId, on),
  dataSettings: (): Promise<DataSettings> => g.GetDataSettings(),
  setDataLimits: (retentionDays: number, maxStorageMB: number): Promise<DataSettings> => g.SetDataLimits(retentionDays, maxStorageMB),
  chooseRecordingsFolder: (): Promise<DataSettings> => g.ChooseRecordingsFolder(),
  resetRecordingsFolder: (): Promise<DataSettings> => g.ResetRecordingsFolder(),
  recordedSeries: (): Promise<SeriesInfo[]> => g.RecordedSeries(),
  seriesIDs: (refs: { connection: string; slaveId: number; function: number; address: number; format: string }[]): Promise<string[]> =>
    g.SeriesIDs(refs),
  saveExportFile: (title: string, name: string, filterName: string, pattern: string, data: string, binary: boolean): Promise<string> =>
    g.SaveExportFile(title, name, filterName, pattern, data, binary),
  querySeries: (id: string, from: number, to: number, maxPoints: number): Promise<Sample[]> => g.QuerySeries(id, from, to, maxPoints),
  exportRecording: (ids: string[], from: number, to: number, format: 'csv' | 'xlsx'): Promise<{ path: string; rows: number }> =>
    g.ExportRecording(ids, from, to, format),
  deleteRecordedSeries: (ids: string[]): Promise<void> => g.DeleteRecordedSeries(ids),
  deleteRecordedRange: (ids: string[], from: number, to: number): Promise<void> => g.DeleteRecordedRange(ids, from, to),

  statistics: (detailId: string): Promise<ConnectionStats[]> => g.Statistics(detailId),
  resetStatistics: (connectionId: string): Promise<void> => g.ResetStatistics(connectionId),
  startIDScan: (r: IDScanRequest): Promise<void> => g.StartIDScan(r),
  startMapScan: (r: MapScanRequest): Promise<void> => g.StartMapScan(r),
  startNetworkScan: (r: NetworkScanRequest): Promise<void> => g.StartNetworkScan(r),
  stopScan: (): Promise<void> => g.StopScan(),
  scanState: (): Promise<ScanState> => g.ScanState(),
  pollsFromMap: (connectionId: string, slaveId: number, ranges: MapRange[]): Promise<Poll[]> => g.PollsFromMap(connectionId, slaveId, ranges),
  echoTest: (connectionId: string, slaveId: number, count: number): Promise<EchoResult> => g.EchoTest(connectionId, slaveId, count),
  diagCounters: (connectionId: string, slaveId: number): Promise<CounterValue[]> => g.DiagCounters(connectionId, slaveId),
  clearDiagCounters: (connectionId: string, slaveId: number): Promise<void> => g.ClearDiagCounters(connectionId, slaveId),
  readDeviceIdentification: (connectionId: string, slaveId: number, code: number): Promise<DeviceID> =>
    g.ReadDeviceIdentification(connectionId, slaveId, code),

  alarms: (): Promise<Alarm[]> => g.Alarms(),
  ackAlarms: (ids: string[]): Promise<void> => g.AckAlarms(ids),
  alarmHistory: (from: number, to: number, limit: number): Promise<AlarmEvent[]> => g.AlarmHistory(from, to, limit),
  exportAlarmHistory: (from: number, to: number): Promise<{ path: string; rows: number }> => g.ExportAlarmHistory(from, to),

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
  onRecording: (cb: (error: string) => void) => EventsOn('recording:status', cb),
  onAlarms: (cb: (a: Alarm[]) => void) => EventsOn('alarms', cb),
  onScan: (cb: (s: ScanState) => void) => EventsOn('scan', cb),
  onWorkspace: (cb: (s: WorkspaceState) => void) => EventsOn('workspace:state', cb),
  onUpdateState: (cb: (s: UpdateState) => void) => EventsOn('update:state', cb),
  onUpdateProgress: (cb: (p: UpdateProgress) => void) => EventsOn('update:progress', cb),
}

/** Turns a rejected binding promise into a display string. */
export const errText = (e: unknown) => (typeof e === 'string' ? e : e instanceof Error ? e.message : String(e))
