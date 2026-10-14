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
}

export interface Cell {
  address: number
  raw: number
  value: string
  span: number
  format: string
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
