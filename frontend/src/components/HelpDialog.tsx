import { useMemo, useState, type ReactNode } from 'react'
import { Modal } from './Modal'

// An offline guide to the Modbus protocol. Example frames are real: the CRC
// and LRC bytes are computed, so they can be typed into a slave and checked.

type Part = 'addr' | 'fc' | 'data' | 'check' | 'mbap' | 'start' | 'end'

const PART_LABEL: Record<Part, string> = {
  addr: 'Slave / unit ID',
  fc: 'Function code',
  data: 'Data',
  check: 'Error check',
  mbap: 'MBAP header',
  start: 'Start',
  end: 'End',
}

/** A frame drawn as coloured byte boxes with a legend. */
function Frame({ title, bytes }: { title: string; bytes: [string, Part, string?][] }) {
  const parts = [...new Set(bytes.map((b) => b[1]))]
  return (
    <figure className="hf">
      <figcaption>{title}</figcaption>
      <div className="hf-bytes">
        {bytes.map(([hex, part, tip], i) => (
          <span key={i} className={`hf-b ${part}`} title={tip ?? PART_LABEL[part]}>{hex}</span>
        ))}
      </div>
      <div className="hf-legend">
        {parts.map((p) => <span key={p}><i className={`hf-b ${p}`} />{PART_LABEL[p]}</span>)}
      </div>
    </figure>
  )
}

function Table({ head, rows }: { head: string[]; rows: ReactNode[][] }) {
  return (
    <table className="tbl ht">
      <thead><tr>{head.map((h) => <th key={h}>{h}</th>)}</tr></thead>
      <tbody>{rows.map((r, i) => <tr key={i}>{r.map((c, j) => <td key={j}>{c}</td>)}</tr>)}</tbody>
    </table>
  )
}

const yes = <span className="ht-yes">✓</span>

interface Topic {
  id: string
  title: string
  /** Extra words for search, besides the title. */
  keywords: string
  body: ReactNode
}

const TOPICS: Topic[] = [
  {
    id: 'overview',
    title: 'What is Modbus?',
    keywords: 'introduction master slave client server request response polling',
    body: (
      <>
        <p>
          Modbus is a simple request/response protocol, published by Modicon in 1979 and now maintained by the Modbus Organization.
          It is used to read and write data in meters, PLCs, drives, sensors and controllers.
        </p>
        <p>
          One device, the <b>master</b> (called <i>client</i> in Modbus TCP), sends a request. The addressed <b>slave</b> (<i>server</i>)
          answers. Slaves never speak on their own, so the master <b>polls</b> them in turn. Modbus Studio is a master; its built-in
          simulator is a slave.
        </p>
        <p>Every request carries the same core, the <b>PDU</b> (protocol data unit):</p>
        <ul>
          <li><b>Function code</b>, 1 byte: what to do, e.g. 03 = read holding registers.</li>
          <li><b>Data</b>: addresses, quantities and values, all <b>big-endian</b> (high byte first).</li>
        </ul>
        <p>
          The transport wraps the PDU in an <b>ADU</b> (application data unit): on a serial line it adds the slave address and a CRC or LRC;
          on TCP/UDP it adds a 7-byte MBAP header instead. See <i>Serial: RTU and ASCII</i> and <i>Modbus TCP and UDP</i>.
        </p>
        <p className="muted">A PDU is at most 253 bytes, which is why one read returns at most 125 registers or 2000 coils.</p>
      </>
    ),
  },
  {
    id: 'data',
    title: 'Data model',
    keywords: 'coils discrete inputs input registers holding registers tables bit 16-bit read only read write',
    body: (
      <>
        <p>A Modbus device exposes four tables. Each has its own address space from 0 to 65535:</p>
        <Table
          head={['Table', 'Size', 'Access', 'Read', 'Write', 'Typical use']}
          rows={[
            ['Coils', '1 bit', 'Read / write', '01', '05, 15', 'Relay outputs, on/off commands'],
            ['Discrete inputs', '1 bit', 'Read only', '02', '—', 'Digital inputs, status flags'],
            ['Input registers', '16 bits', 'Read only', '04', '—', 'Measurements (voltage, current…)'],
            ['Holding registers', '16 bits', 'Read / write', '03', '06, 16, 23', 'Setpoints, configuration, and often measurements too'],
          ]}
        />
        <p>
          A register holds one 16-bit word (0–65535, or −32768–32767 signed). Larger values such as 32-bit floats use
          two or four consecutive registers; see <i>Data types and byte order</i>.
        </p>
        <p className="muted">Many devices put everything in holding registers. The device's register map says which table and function to use.</p>
      </>
    ),
  },
  {
    id: 'addressing',
    title: 'Addressing (0 or 1, 4xxxx)',
    keywords: 'address offset base 0 base 1 plc protocol modicon 40001 30001 10001 00001 off by one',
    body: (
      <>
        <p>
          On the wire, addresses start at <b>0</b> (the <i>protocol address</i>). Many manuals instead use 1-based <b>Modicon numbers</b>
          whose first digit names the table:
        </p>
        <Table
          head={['Manual says', 'Table', 'Function', 'Address on the wire']}
          rows={[
            ['00001–09999', 'Coils', '01', 'number − 1'],
            ['10001–19999', 'Discrete inputs', '02', 'number − 10001'],
            ['30001–39999', 'Input registers', '04', 'number − 30001'],
            ['40001–49999', 'Holding registers', '03', 'number − 40001'],
          ]}
        />
        <p>
          Example: <b>40108</b> means holding register, protocol address <b>107</b>, read with function 03.
          Six-digit forms (400108) work the same way and allow addresses above 9999.
        </p>
        <p>
          In a poll window, <b>PLC addresses (base 1)</b> shows and accepts 1-based numbers; <b>Protocol addresses (base 0)</b> shows the
          wire address. If every value seems shifted by one register, the base is wrong.
        </p>
      </>
    ),
  },
  {
    id: 'functions',
    title: 'Function codes',
    keywords: 'function code fc 01 02 03 04 05 06 08 15 16 23 43 14 limits quantity max',
    body: (
      <>
        <Table
          head={['Code', 'Name', 'Limit per request', 'Modbus Studio']}
          rows={[
            ['01', 'Read Coils', '2000 coils', yes],
            ['02', 'Read Discrete Inputs', '2000 inputs', yes],
            ['03', 'Read Holding Registers', '125 registers', yes],
            ['04', 'Read Input Registers', '125 registers', yes],
            ['05', 'Write Single Coil', 'FF00 = on, 0000 = off', yes],
            ['06', 'Write Single Register', '1 register', yes],
            ['07', 'Read Exception Status', 'serial only', ''],
            ['08', 'Diagnostics', 'echo, counters (serial)', yes],
            ['11', 'Get Comm Event Counter', 'serial only', ''],
            ['15', 'Write Multiple Coils', '1968 coils', yes],
            ['16', 'Write Multiple Registers', '123 registers', yes],
            ['17', 'Report Server ID', '', ''],
            ['22', 'Mask Write Register', 'AND / OR mask', ''],
            ['23', 'Read/Write Multiple Registers', 'read 125, write 121', yes],
            ['43 / 14', 'Read Device Identification', 'vendor, product, version', yes],
          ]}
        />
        <p>
          A request for more than the limit is answered with exception 03. A slave may support only some functions; an unsupported one
          gives exception 01.
        </p>
      </>
    ),
  },
  {
    id: 'rtu',
    title: 'Serial: RTU and ASCII',
    keywords: 'rtu ascii crc lrc frame timing 3.5 character silent interval broadcast slave address 247 colon',
    body: (
      <>
        <h5>Modbus RTU</h5>
        <p>
          Binary framing, the most common on serial lines. A frame is <b>slave address</b> (1 byte) + <b>PDU</b> + <b>CRC-16</b> (2 bytes,
          low byte first). Frames are separated by at least <b>3.5 character times</b> of silence, and a gap of more than 1.5
          character times inside a frame breaks it. Above 19200 baud fixed gaps of 1.75 ms and 0.75 ms are used.
        </p>
        <Frame
          title="Request: read 2 holding registers from address 0 of slave 1"
          bytes={[['01', 'addr'], ['03', 'fc', 'Read Holding Registers'], ['00', 'data', 'Start address (high)'], ['00', 'data', 'Start address (low)'], ['00', 'data', 'Quantity (high)'], ['02', 'data', 'Quantity (low) = 2'], ['C4', 'check', 'CRC low'], ['0B', 'check', 'CRC high']]}
        />
        <Frame
          title="Response: 4 data bytes = 42F6 E979 (123.456 as a Float AB CD)"
          bytes={[['01', 'addr'], ['03', 'fc'], ['04', 'data', 'Byte count'], ['42', 'data'], ['F6', 'data'], ['E9', 'data'], ['79', 'data'], ['80', 'check', 'CRC low'], ['0B', 'check', 'CRC high']]}
        />
        <Frame
          title="Write single register: address 1 = 3"
          bytes={[['01', 'addr'], ['06', 'fc', 'Write Single Register'], ['00', 'data'], ['01', 'data', 'Address 1'], ['00', 'data'], ['03', 'data', 'Value 3'], ['98', 'check'], ['0B', 'check']]}
        />
        <p>
          Slave addresses are <b>1–247</b>. Address <b>0</b> is broadcast: every slave executes a write and none replies.
          The CRC uses polynomial 0xA001 (reflected 0x8005), starting value 0xFFFF.
        </p>
        <h5>Modbus ASCII</h5>
        <p>
          The same content written as hexadecimal text: a <code>:</code>, two characters per byte, an <b>LRC</b> (two's complement of
          the byte sum) and <code>CR LF</code>. Slower but tolerant of long gaps; usually 7 data bits.
        </p>
        <Frame
          title="The same read request in ASCII"
          bytes={[[':', 'start'], ['01', 'addr'], ['03', 'fc'], ['00', 'data'], ['00', 'data'], ['00', 'data'], ['02', 'data'], ['FA', 'check', 'LRC'], ['CR LF', 'end']]}
        />
      </>
    ),
  },
  {
    id: 'tcp',
    title: 'Modbus TCP and UDP',
    keywords: 'tcp ip udp mbap transaction id protocol id length unit id port 502 gateway rtu over tcp ethernet',
    body: (
      <>
        <p>
          Over Ethernet the PDU is preceded by a 7-byte <b>MBAP header</b> and there is no CRC (TCP already checks the data).
          The default port is <b>502</b>.
        </p>
        <Frame
          title="The same read request over Modbus TCP"
          bytes={[['00', 'mbap', 'Transaction ID (high)'], ['01', 'mbap', 'Transaction ID (low): echoed in the reply'], ['00', 'mbap', 'Protocol ID'], ['00', 'mbap', 'Protocol ID = 0'], ['00', 'mbap', 'Length (high)'], ['06', 'mbap', 'Length: bytes that follow'], ['01', 'addr', 'Unit ID'], ['03', 'fc'], ['00', 'data'], ['00', 'data'], ['00', 'data'], ['02', 'data']]}
        />
        <ul>
          <li><b>Transaction ID</b> matches replies to requests.</li>
          <li><b>Length</b> counts the unit ID plus the PDU.</li>
          <li>
            <b>Unit ID</b> selects a slave behind a gateway (TCP to RS-485). Devices on the network themselves usually accept
            1, 0 or 255.
          </li>
        </ul>
        <p><b>Modbus UDP</b> uses the same frame without a connection: lighter, but lost packets are only detected by a timeout.</p>
        <p>
          <b>RTU over TCP / UDP</b> (and ASCII over TCP) send the serial frame, CRC included, through a socket with no MBAP header.
          Some serial-to-Ethernet converters work this way; if a converter answers nothing in Modbus TCP mode, try RTU over TCP.
        </p>
      </>
    ),
  },
  {
    id: 'exceptions',
    title: 'Exception responses',
    keywords: 'exception error code illegal function illegal data address illegal data value device failure busy gateway 0x80',
    body: (
      <>
        <p>
          A slave that understands a request but cannot carry it out replies with the function code <b>+ 0x80</b> and an exception code.
        </p>
        <Frame title="Exception: function 03 failed with code 02 (illegal data address)" bytes={[['01', 'addr'], ['83', 'fc', '03 + 80'], ['02', 'data', 'Exception code'], ['C0', 'check'], ['F1', 'check']]} />
        <Table
          head={['Code', 'Name', 'Usual cause']}
          rows={[
            ['01', 'Illegal function', 'The slave does not support this function (try 03 instead of 04, or the reverse)'],
            ['02', 'Illegal data address', 'Address or address + quantity is outside the map; check base 0/1 and the quantity'],
            ['03', 'Illegal data value', 'Quantity above the limit, or a value the device refuses'],
            ['04', 'Server device failure', 'Internal error in the slave'],
            ['05', 'Acknowledge', 'Accepted, still processing a long operation'],
            ['06', 'Server device busy', 'Try again later'],
            ['08', 'Memory parity error', 'File record access failed'],
            ['0A', 'Gateway path unavailable', 'Gateway misconfigured or overloaded'],
            ['0B', 'Gateway target failed to respond', 'The serial slave behind the gateway did not answer: wrong unit ID or wiring'],
          ]}
        />
        <p className="muted">No reply at all is not an exception: it is a timeout. See <i>Troubleshooting</i>.</p>
      </>
    ),
  },
  {
    id: 'types',
    title: 'Data types and byte order',
    keywords: 'float long double int32 uint32 int64 byte order endian word swap ab cd cd ab ba dc dc ba signed unsigned hex binary',
    body: (
      <>
        <p>
          Modbus defines only 16-bit registers, sent high byte first. Wider values span several registers, and devices disagree on the
          order. Modbus Studio names the order by bytes, A being the most significant:
        </p>
        <Table
          head={['Order', 'Also called', 'Float 123.456 in two registers']}
          rows={[
            ['AB CD', 'Big-endian, high word first', <span className="mono">42F6 E979</span>],
            ['CD AB', 'Word swap, little-endian words (very common)', <span className="mono">E979 42F6</span>],
            ['BA DC', 'Byte swap', <span className="mono">F642 79E9</span>],
            ['DC BA', 'Little-endian', <span className="mono">79E9 F642</span>],
          ]}
        />
        <Table
          head={['Format', 'Registers', 'Range']}
          rows={[
            ['Signed / Unsigned', '1', '−32768…32767 / 0…65535'],
            ['Hex / Binary', '1', 'The raw 16 bits'],
            ['Long / Unsigned Long', '2', '±2.1 × 10⁹ / 0…4.29 × 10⁹'],
            ['Float', '2', 'IEEE 754 single, about 7 digits'],
            ['Long64 / Double', '4', '64-bit integer / IEEE 754 double'],
          ]}
        />
        <p>
          If a float shows a huge or tiny nonsense number, try the other byte orders, and check that you start on the right register
          (an odd offset splits the value). Change the format for a whole window with its format button, or for one register with
          right-click → Format.
        </p>
      </>
    ),
  },
  {
    id: 'serial',
    title: 'RS-485 wiring and settings',
    keywords: 'rs485 rs-485 rs232 wiring termination 120 ohm bias a b d+ d- baud parity stop bits daisy chain ground',
    body: (
      <>
        <ul>
          <li><b>RS-485</b> is the usual physical layer: two wires (A/B, also D−/D+) shared by up to 32 standard devices, up to about 1200 m at low baud rates.</li>
          <li>Wire as a <b>daisy chain</b>, not a star. Put a <b>120 Ω terminator</b> at both ends of the line, and connect the common/ground.</li>
          <li>Makers label A and B inconsistently. If nothing answers, swapping the two wires is a safe first test.</li>
          <li>Every device on the line needs the same <b>baud rate, data bits, parity and stop bits</b>, and a unique slave address.</li>
          <li>The standard default is 19200 baud, 8 data bits, even parity, 1 stop bit (8E1). Many devices ship with 9600 8N1.</li>
          <li>Only one master per line. Two masters cause collisions, seen as CRC errors.</li>
        </ul>
        <p>
          In Modbus Studio several poll windows can share one connection: requests are queued, never sent at the same time.
          <b> Delay between polls</b> (Connection) gives slow slaves time to switch their transmitter off.
        </p>
      </>
    ),
  },
  {
    id: 'trouble',
    title: 'Troubleshooting',
    keywords: 'timeout no response crc error wrong values garbage connection refused unit id problems fix diagnose',
    body: (
      <Table
        head={['Symptom', 'Check']}
        rows={[
          ['Timeout (no reply)', 'Slave ID; baud rate, parity and stop bits; A/B wires swapped; correct COM port; device powered and in Modbus mode; for TCP, IP address and unit ID'],
          ['CRC errors', 'Baud rate mismatch, missing termination, noise or long unshielded cable, a second master on the line'],
          ['Exception 01', 'Use the other read function (03 ↔ 04) or the one the register map names'],
          ['Exception 02', 'Address outside the map: base 0/1 offset, or the quantity reaches past the last register'],
          ['Values shifted by one register', 'PLC (base 1) vs protocol (base 0) addressing'],
          ['Nonsense 32-bit values', 'Byte order (try CD AB), start register, signed vs unsigned'],
          ['Connection refused (TCP)', 'Port (502), firewall, the device accepts only one or a few connections at a time'],
          ['Some reads time out', 'Increase the timeout or the delay between polls; lower the scan rate; read fewer registers per window'],
        ]}
      />
    ),
  },
  {
    id: 'app',
    title: 'Using Modbus Studio',
    keywords: 'how to start poll window simulator traffic diagnostics scan chart record alarms profiles tags',
    body: (
      <ul>
        <li><b>Connection</b>: set up a serial port or an IP address. Several windows can share it.</li>
        <li><b>Poll window</b>: slave ID, function, start address, quantity and scan rate. Start, Stop, or read once.</li>
        <li><b>Format button</b> in a window's header: change how its values are shown, on the fly.</li>
        <li><b>Double-click</b> a coil or holding register to write it; <b>right-click</b> for tag, chart and per-register format.</li>
        <li><b>Traffic log</b>: every frame sent and received, in hex, with timestamps. The quickest way to see what is wrong.</li>
        <li><b>Diagnostics</b>: statistics, slave ID scan, register map probe, TCP network scan, 08 echo and 43/14 identification.</li>
        <li><b>Simulator</b>: a built-in slave for trying things out without hardware.</li>
        <li><b>Tags, profiles, alarms, charts, recording and test sequences</b> build on poll windows.</li>
      </ul>
    ),
  },
]

/** The Modbus protocol guide: topics on the left, searchable. */
export function HelpDialog({ onClose }: { onClose: () => void }) {
  const [topic, setTopic] = useState(TOPICS[0].id)
  const [search, setSearch] = useState('')
  const shown = useMemo(() => {
    const words = search.toLowerCase().split(/\s+/).filter(Boolean)
    if (!words.length) return TOPICS
    return TOPICS.filter((t) => {
      const hay = `${t.title} ${t.keywords}`.toLowerCase()
      return words.every((w) => hay.includes(w))
    })
  }, [search])
  const current = shown.find((t) => t.id === topic) ?? shown[0]
  return (
    <Modal title="Modbus protocol guide" onClose={onClose} width={900} footer={<button className="btn primary" onClick={onClose}>Close</button>}>
      <div className="prefs help">
        <nav className="prefs-nav">
          <input type="search" placeholder="Search…" value={search} onChange={(e) => setSearch(e.target.value)} aria-label="Search the guide" />
          {shown.map((t) => (
            <button key={t.id} className={current?.id === t.id ? 'on' : ''} onClick={() => setTopic(t.id)}>{t.title}</button>
          ))}
          {shown.length === 0 && <span className="muted" style={{ padding: '6px 10px' }}>No topic matches.</span>}
        </nav>
        <article className="prefs-page help-page">
          {current && (
            <>
              <h4>{current.title}</h4>
              {current.body}
            </>
          )}
        </article>
      </div>
    </Modal>
  )
}
