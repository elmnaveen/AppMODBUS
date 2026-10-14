export type Mode = 'rtu' | 'tcp' | 'rtu-tcp' | 'rtu-udp'

export interface ConnectionConfig {
  id: string
  name: string
  mode: Mode
  serialPort: string
  baudRate: number
  dataBits: number
  parity: 'N' | 'E' | 'O'
  stopBits: number
  host: string
  port: number
  timeoutMs: number
  delayMs: number
}

export interface ConnectionInfo extends ConnectionConfig {
  summary: string
  connected: boolean
  error: string
}

export interface PollConfig {
  id: string
  name: string
  connectionId: string
  slaveId: number
  function: number
  address: number
  quantity: number
  scanRateMs: number
  format: string
  plcAddressing: boolean
  cellFormats: Record<string, string>
  tags: Record<string, Tag>
}

/** Register name and scaling: shown = raw × gain + offset. */
export interface Tag {
  name: string
  gain: number
  offset: number
  unit: string
  decimals: number // -1 = automatic
  note?: string
}

export interface Cell {
  address: number
  raw: number
  value: string
  span: number
  format: string
  name?: string
  unit?: string
  rawText?: string
}

export interface PollState {
  id: string
  running: boolean
  tx: number
  errors: number
  valid: number
  lastError: string
  responseMs: number
  updatedAt: number
  hasData: boolean
  cells: Cell[]
}

export interface Poll {
  config: PollConfig
  state: PollState
}

export interface FormatInfo {
  id: string
  label: string
  registers: number
  group: string
}

export interface TrafficEntry {
  seq: number
  time: number
  source: string
  dir: 'Tx' | 'Rx'
  hex: string
  len: number
}

export interface WriteRequest {
  connectionId: string
  slaveId: number
  function: number
  address: number
  values: number[]
}

export interface SimConfig {
  mode: Mode
  serialPort: string
  baudRate: number
  dataBits: number
  parity: string
  stopBits: number
  host: string
  port: number
  slaveId: number
  animate: boolean
}

export interface SimStatus {
  running: boolean
  config: SimConfig
  requests: number
  clients: number
  error: string
}

export type SimTable = 'coils' | 'discrete' | 'holding' | 'input'

export const MODE_LABELS: Record<Mode, string> = {
  rtu: 'Modbus RTU (serial)',
  tcp: 'Modbus TCP/IP',
  'rtu-tcp': 'RTU over TCP',
  'rtu-udp': 'RTU over UDP',
}

export const MODE_SHORT: Record<Mode, string> = {
  rtu: 'RTU (serial)',
  tcp: 'TCP/IP',
  'rtu-tcp': 'RTU over TCP',
  'rtu-udp': 'RTU over UDP',
}

export const FUNCTIONS: { code: number; label: string }[] = [
  { code: 1, label: '01 Read Coils (0x)' },
  { code: 2, label: '02 Read Discrete Inputs (1x)' },
  { code: 3, label: '03 Read Holding Registers (4x)' },
  { code: 4, label: '04 Read Input Registers (3x)' },
]

export const isBitFunction = (fc: number) => fc === 1 || fc === 2
export const isWritable = (fc: number) => fc === 1 || fc === 3
export const fc2 = (fc: number) => fc.toString().padStart(2, '0')

// ---------- application updates ----------

export interface AppInfo {
  version: string
  commit: string
  os: string
  arch: string
  repo: string
  canInstall: boolean
}

export interface ReleaseAsset {
  name: string
  url: string
  size: number
}

export interface Release {
  version: string
  tag: string
  name: string
  notes: string
  pageUrl: string
  prerelease: boolean
  publishedAt: string
  asset: ReleaseAsset
  installer: ReleaseAsset | null
}

export type UpdateStatus = 'idle' | 'checking' | 'uptodate' | 'available' | 'downloading' | 'installing' | 'error'

export interface UpdateState {
  status: UpdateStatus
  release: Release | null
  error: string
  checkedAt: number
  skipped: boolean
}

export interface UpdateSettings {
  updateChannel: 'stable' | 'beta'
  autoCheckUpdates: boolean
  skippedVersion: string
  lastUpdateCheckAt: number
}

export interface UpdateProgress {
  done: number
  total: number
}

// ---------- workspace ----------

export interface Layout {
  view: 'tabs' | 'tile'
  activeId: string
  showTraffic: boolean
  showSimulator: boolean
}

export interface WorkspaceState {
  path: string
  name: string
  dirty: boolean
  recent: string[]
}

export interface MissingPort {
  connectionId: string
  name: string
  port: string
}

export interface LoadResult {
  state: WorkspaceState
  layout: Layout
  charts: unknown
  warnings: string[]
  missingPorts: MissingPort[]
  ports: string[]
}

// ---------- device profiles ----------

export interface ProfileRegister {
  address: number
  function: number
  name: string
  format: string
  gain: number
  offset: number
  unit: string
  decimals: number
  note?: string
}

export interface Profile {
  format: string
  version: number
  name: string
  manufacturer: string
  model: string
  description: string
  defaultSlaveId: number
  registers: ProfileRegister[]
}

export type ProfileSource = 'bundled' | 'user' | 'shared'

export interface ProfileSummary {
  id: string
  source: ProfileSource
  name: string
  manufacturer: string
  model: string
  registers: number
  path: string
  error?: string
}

export type ImportField = 'address' | 'function' | 'name' | 'format' | 'gain' | 'offset' | 'unit' | 'decimals' | 'note'

export interface ImportOptions {
  headerRow: number
  mapping: Partial<Record<ImportField, number>>
  addressMode: 'zero' | 'one' | 'modicon'
  defaultFunction: number
  byteOrder: 'abcd' | 'cdab' | 'badc' | 'dcba'
}

export interface ImportPreview {
  path: string
  fileName: string
  rows: string[][]
  totalRows: number
  options: ImportOptions
  fields: ImportField[]
}

export interface ImportResult {
  profile: Profile
  warnings: string[]
}

export interface ProfileFolders {
  user: string
  shared: string
}
