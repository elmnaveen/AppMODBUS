export namespace applog {
	
	export class Entry {
	    id: number;
	    time: number;
	    level: string;
	    source: string;
	    message: string;
	
	    static createFrom(source: any = {}) {
	        return new Entry(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.time = source["time"];
	        this.level = source["level"];
	        this.source = source["source"];
	        this.message = source["message"];
	    }
	}

}

export namespace datafmt {
	
	export class Info {
	    id: string;
	    label: string;
	    registers: number;
	    group: string;
	
	    static createFrom(source: any = {}) {
	        return new Info(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.label = source["label"];
	        this.registers = source["registers"];
	        this.group = source["group"];
	    }
	}

}

export namespace engine {
	
	export class Alarm {
	    id: string;
	    pollId: string;
	    address: number;
	    window: string;
	    name: string;
	    unit: string;
	    level: string;
	    severity: string;
	    value: number;
	    limit: number;
	    message: string;
	    active: boolean;
	    acked: boolean;
	    since: number;
	    clearedAt: number;
	    ackedAt: number;
	
	    static createFrom(source: any = {}) {
	        return new Alarm(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.pollId = source["pollId"];
	        this.address = source["address"];
	        this.window = source["window"];
	        this.name = source["name"];
	        this.unit = source["unit"];
	        this.level = source["level"];
	        this.severity = source["severity"];
	        this.value = source["value"];
	        this.limit = source["limit"];
	        this.message = source["message"];
	        this.active = source["active"];
	        this.acked = source["acked"];
	        this.since = source["since"];
	        this.clearedAt = source["clearedAt"];
	        this.ackedAt = source["ackedAt"];
	    }
	}
	export class AlarmEvent {
	    t: number;
	    kind: string;
	    id: string;
	    window: string;
	    name: string;
	    level: string;
	    value: number;
	    limit: number;
	    unit: string;
	    message: string;
	
	    static createFrom(source: any = {}) {
	        return new AlarmEvent(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.t = source["t"];
	        this.kind = source["kind"];
	        this.id = source["id"];
	        this.window = source["window"];
	        this.name = source["name"];
	        this.level = source["level"];
	        this.value = source["value"];
	        this.limit = source["limit"];
	        this.unit = source["unit"];
	        this.message = source["message"];
	    }
	}
	export class AlarmLimits {
	    enabled: boolean;
	    hiHi?: number;
	    hi?: number;
	    lo?: number;
	    loLo?: number;
	    deadband: number;
	    onState?: number;
	
	    static createFrom(source: any = {}) {
	        return new AlarmLimits(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.enabled = source["enabled"];
	        this.hiHi = source["hiHi"];
	        this.hi = source["hi"];
	        this.lo = source["lo"];
	        this.loLo = source["loLo"];
	        this.deadband = source["deadband"];
	        this.onState = source["onState"];
	    }
	}
	export class Cell {
	    address: number;
	    raw: number;
	    value: string;
	    span: number;
	    format: string;
	    name?: string;
	    unit?: string;
	    rawText?: string;
	    num: number;
	    hasNum: boolean;
	    alarm?: string;
	
	    static createFrom(source: any = {}) {
	        return new Cell(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.address = source["address"];
	        this.raw = source["raw"];
	        this.value = source["value"];
	        this.span = source["span"];
	        this.format = source["format"];
	        this.name = source["name"];
	        this.unit = source["unit"];
	        this.rawText = source["rawText"];
	        this.num = source["num"];
	        this.hasNum = source["hasNum"];
	        this.alarm = source["alarm"];
	    }
	}
	export class ConnectionConfig {
	    id: string;
	    name: string;
	    mode: string;
	    serialPort: string;
	    baudRate: number;
	    dataBits: number;
	    parity: string;
	    stopBits: number;
	    host: string;
	    port: number;
	    timeoutMs: number;
	    delayMs: number;
	
	    static createFrom(source: any = {}) {
	        return new ConnectionConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.mode = source["mode"];
	        this.serialPort = source["serialPort"];
	        this.baudRate = source["baudRate"];
	        this.dataBits = source["dataBits"];
	        this.parity = source["parity"];
	        this.stopBits = source["stopBits"];
	        this.host = source["host"];
	        this.port = source["port"];
	        this.timeoutMs = source["timeoutMs"];
	        this.delayMs = source["delayMs"];
	    }
	}
	export class ConnectionInfo {
	    id: string;
	    name: string;
	    mode: string;
	    serialPort: string;
	    baudRate: number;
	    dataBits: number;
	    parity: string;
	    stopBits: number;
	    host: string;
	    port: number;
	    timeoutMs: number;
	    delayMs: number;
	    summary: string;
	    connected: boolean;
	    error: string;
	
	    static createFrom(source: any = {}) {
	        return new ConnectionInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.mode = source["mode"];
	        this.serialPort = source["serialPort"];
	        this.baudRate = source["baudRate"];
	        this.dataBits = source["dataBits"];
	        this.parity = source["parity"];
	        this.stopBits = source["stopBits"];
	        this.host = source["host"];
	        this.port = source["port"];
	        this.timeoutMs = source["timeoutMs"];
	        this.delayMs = source["delayMs"];
	        this.summary = source["summary"];
	        this.connected = source["connected"];
	        this.error = source["error"];
	    }
	}
	export class PollStats {
	    id: string;
	    name: string;
	    stats: Stats;
	
	    static createFrom(source: any = {}) {
	        return new PollStats(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.stats = this.convertValues(source["stats"], Stats);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class RTSample {
	    t: number;
	    ms: number;
	    ok: boolean;
	
	    static createFrom(source: any = {}) {
	        return new RTSample(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.t = source["t"];
	        this.ms = source["ms"];
	        this.ok = source["ok"];
	    }
	}
	export class HistBin {
	    from: number;
	    to: number;
	    count: number;
	
	    static createFrom(source: any = {}) {
	        return new HistBin(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.from = source["from"];
	        this.to = source["to"];
	        this.count = source["count"];
	    }
	}
	export class Stats {
	    since: number;
	    requests: number;
	    ok: number;
	    timeouts: number;
	    crc: number;
	    exceptions: number;
	    invalid: number;
	    ioErrors: number;
	    byCode: Record<string, number>;
	    minMs: number;
	    avgMs: number;
	    maxMs: number;
	    p50Ms: number;
	    p95Ms: number;
	    lastError: string;
	    lastAt: number;
	    histogram?: HistBin[];
	    recent?: RTSample[];
	
	    static createFrom(source: any = {}) {
	        return new Stats(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.since = source["since"];
	        this.requests = source["requests"];
	        this.ok = source["ok"];
	        this.timeouts = source["timeouts"];
	        this.crc = source["crc"];
	        this.exceptions = source["exceptions"];
	        this.invalid = source["invalid"];
	        this.ioErrors = source["ioErrors"];
	        this.byCode = source["byCode"];
	        this.minMs = source["minMs"];
	        this.avgMs = source["avgMs"];
	        this.maxMs = source["maxMs"];
	        this.p50Ms = source["p50Ms"];
	        this.p95Ms = source["p95Ms"];
	        this.lastError = source["lastError"];
	        this.lastAt = source["lastAt"];
	        this.histogram = this.convertValues(source["histogram"], HistBin);
	        this.recent = this.convertValues(source["recent"], RTSample);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class ConnectionStats {
	    id: string;
	    name: string;
	    stats: Stats;
	    polls: PollStats[];
	
	    static createFrom(source: any = {}) {
	        return new ConnectionStats(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.stats = this.convertValues(source["stats"], Stats);
	        this.polls = this.convertValues(source["polls"], PollStats);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	
	export class IDResult {
	    slaveId: number;
	    status: string;
	    exception: string;
	    ms: number;
	
	    static createFrom(source: any = {}) {
	        return new IDResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.slaveId = source["slaveId"];
	        this.status = source["status"];
	        this.exception = source["exception"];
	        this.ms = source["ms"];
	    }
	}
	export class IDScanRequest {
	    connectionId: string;
	    from: number;
	    to: number;
	    function: number;
	    address: number;
	    timeoutMs: number;
	
	    static createFrom(source: any = {}) {
	        return new IDScanRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.connectionId = source["connectionId"];
	        this.from = source["from"];
	        this.to = source["to"];
	        this.function = source["function"];
	        this.address = source["address"];
	        this.timeoutMs = source["timeoutMs"];
	    }
	}
	export class MapRange {
	    function: number;
	    from: number;
	    to: number;
	
	    static createFrom(source: any = {}) {
	        return new MapRange(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.function = source["function"];
	        this.from = source["from"];
	        this.to = source["to"];
	    }
	}
	export class MapScanRequest {
	    connectionId: string;
	    slaveId: number;
	    functions: number[];
	    from: number;
	    to: number;
	    timeoutMs: number;
	
	    static createFrom(source: any = {}) {
	        return new MapScanRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.connectionId = source["connectionId"];
	        this.slaveId = source["slaveId"];
	        this.functions = source["functions"];
	        this.from = source["from"];
	        this.to = source["to"];
	        this.timeoutMs = source["timeoutMs"];
	    }
	}
	export class NetResult {
	    host: string;
	    port: number;
	    units: number[];
	    status: string;
	    exception: string;
	    ms: number;
	    identity: string[];
	
	    static createFrom(source: any = {}) {
	        return new NetResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.host = source["host"];
	        this.port = source["port"];
	        this.units = source["units"];
	        this.status = source["status"];
	        this.exception = source["exception"];
	        this.ms = source["ms"];
	        this.identity = source["identity"];
	    }
	}
	export class NetworkScanRequest {
	    range: string;
	    port: number;
	    units: string;
	    timeoutMs: number;
	    identify: boolean;
	
	    static createFrom(source: any = {}) {
	        return new NetworkScanRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.range = source["range"];
	        this.port = source["port"];
	        this.units = source["units"];
	        this.timeoutMs = source["timeoutMs"];
	        this.identify = source["identify"];
	    }
	}
	export class PollState {
	    id: string;
	    running: boolean;
	    tx: number;
	    errors: number;
	    valid: number;
	    lastError: string;
	    responseMs: number;
	    updatedAt: number;
	    hasData: boolean;
	    cells: Cell[];
	
	    static createFrom(source: any = {}) {
	        return new PollState(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.running = source["running"];
	        this.tx = source["tx"];
	        this.errors = source["errors"];
	        this.valid = source["valid"];
	        this.lastError = source["lastError"];
	        this.responseMs = source["responseMs"];
	        this.updatedAt = source["updatedAt"];
	        this.hasData = source["hasData"];
	        this.cells = this.convertValues(source["cells"], Cell);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class Tag {
	    name: string;
	    gain: number;
	    offset: number;
	    unit: string;
	    decimals: number;
	    note?: string;
	    alarm?: AlarmLimits;
	
	    static createFrom(source: any = {}) {
	        return new Tag(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.gain = source["gain"];
	        this.offset = source["offset"];
	        this.unit = source["unit"];
	        this.decimals = source["decimals"];
	        this.note = source["note"];
	        this.alarm = this.convertValues(source["alarm"], AlarmLimits);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class PollConfig {
	    id: string;
	    name: string;
	    connectionId: string;
	    slaveId: number;
	    function: number;
	    address: number;
	    quantity: number;
	    scanRateMs: number;
	    format: string;
	    plcAddressing: boolean;
	    cellFormats: Record<number, string>;
	    tags: Record<number, Tag>;
	    record: boolean;
	    commAlarm: boolean;
	
	    static createFrom(source: any = {}) {
	        return new PollConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.connectionId = source["connectionId"];
	        this.slaveId = source["slaveId"];
	        this.function = source["function"];
	        this.address = source["address"];
	        this.quantity = source["quantity"];
	        this.scanRateMs = source["scanRateMs"];
	        this.format = source["format"];
	        this.plcAddressing = source["plcAddressing"];
	        this.cellFormats = source["cellFormats"];
	        this.tags = this.convertValues(source["tags"], Tag, true);
	        this.record = source["record"];
	        this.commAlarm = source["commAlarm"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class Poll {
	    config: PollConfig;
	    state: PollState;
	
	    static createFrom(source: any = {}) {
	        return new Poll(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.config = this.convertValues(source["config"], PollConfig);
	        this.state = this.convertValues(source["state"], PollState);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	
	
	
	
	export class ReadWriteRequest {
	    connectionId: string;
	    slaveId: number;
	    readAddress: number;
	    readQuantity: number;
	    writeAddress: number;
	    values: number[];
	
	    static createFrom(source: any = {}) {
	        return new ReadWriteRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.connectionId = source["connectionId"];
	        this.slaveId = source["slaveId"];
	        this.readAddress = source["readAddress"];
	        this.readQuantity = source["readQuantity"];
	        this.writeAddress = source["writeAddress"];
	        this.values = source["values"];
	    }
	}
	export class ScanState {
	    kind: string;
	    running: boolean;
	    done: number;
	    total: number;
	    phase: string;
	    started: number;
	    finished: number;
	    error: string;
	    ids: IDResult[];
	    ranges: MapRange[];
	    unsupported: number[];
	    hosts: NetResult[];
	    slaveId: number;
	    connectionId: string;
	    seq: number;
	
	    static createFrom(source: any = {}) {
	        return new ScanState(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.kind = source["kind"];
	        this.running = source["running"];
	        this.done = source["done"];
	        this.total = source["total"];
	        this.phase = source["phase"];
	        this.started = source["started"];
	        this.finished = source["finished"];
	        this.error = source["error"];
	        this.ids = this.convertValues(source["ids"], IDResult);
	        this.ranges = this.convertValues(source["ranges"], MapRange);
	        this.unsupported = source["unsupported"];
	        this.hosts = this.convertValues(source["hosts"], NetResult);
	        this.slaveId = source["slaveId"];
	        this.connectionId = source["connectionId"];
	        this.seq = source["seq"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class SeqStep {
	    kind: string;
	    label: string;
	    slaveId: number;
	    function: number;
	    address: number;
	    format: string;
	    gain: number;
	    offset: number;
	    check: string;
	    value: number;
	    min: number;
	    max: number;
	    tolerance: number;
	    values: string;
	    waitMs: number;
	    text: string;
	
	    static createFrom(source: any = {}) {
	        return new SeqStep(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.kind = source["kind"];
	        this.label = source["label"];
	        this.slaveId = source["slaveId"];
	        this.function = source["function"];
	        this.address = source["address"];
	        this.format = source["format"];
	        this.gain = source["gain"];
	        this.offset = source["offset"];
	        this.check = source["check"];
	        this.value = source["value"];
	        this.min = source["min"];
	        this.max = source["max"];
	        this.tolerance = source["tolerance"];
	        this.values = source["values"];
	        this.waitMs = source["waitMs"];
	        this.text = source["text"];
	    }
	}
	export class Sequence {
	    id: string;
	    name: string;
	    connectionId: string;
	    slaveId: number;
	    repeat: number;
	    stopOnFail: boolean;
	    stepDelayMs: number;
	    steps: SeqStep[];
	
	    static createFrom(source: any = {}) {
	        return new Sequence(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.connectionId = source["connectionId"];
	        this.slaveId = source["slaveId"];
	        this.repeat = source["repeat"];
	        this.stopOnFail = source["stopOnFail"];
	        this.stepDelayMs = source["stepDelayMs"];
	        this.steps = this.convertValues(source["steps"], SeqStep);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class StepResult {
	    run: number;
	    index: number;
	    label: string;
	    status: string;
	    value: string;
	    expect: string;
	    message: string;
	    ms: number;
	    time: number;
	
	    static createFrom(source: any = {}) {
	        return new StepResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.run = source["run"];
	        this.index = source["index"];
	        this.label = source["label"];
	        this.status = source["status"];
	        this.value = source["value"];
	        this.expect = source["expect"];
	        this.message = source["message"];
	        this.ms = source["ms"];
	        this.time = source["time"];
	    }
	}
	export class SequenceRun {
	    sequenceId: string;
	    name: string;
	    connection: string;
	    running: boolean;
	    prompt: string;
	    run: number;
	    runs: number;
	    step: number;
	    started: number;
	    finished: number;
	    verdict: string;
	    error: string;
	    passed: number;
	    failed: number;
	    errors: number;
	    results: StepResult[];
	    seq: number;
	
	    static createFrom(source: any = {}) {
	        return new SequenceRun(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.sequenceId = source["sequenceId"];
	        this.name = source["name"];
	        this.connection = source["connection"];
	        this.running = source["running"];
	        this.prompt = source["prompt"];
	        this.run = source["run"];
	        this.runs = source["runs"];
	        this.step = source["step"];
	        this.started = source["started"];
	        this.finished = source["finished"];
	        this.verdict = source["verdict"];
	        this.error = source["error"];
	        this.passed = source["passed"];
	        this.failed = source["failed"];
	        this.errors = source["errors"];
	        this.results = this.convertValues(source["results"], StepResult);
	        this.seq = source["seq"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	
	
	
	export class WriteRequest {
	    connectionId: string;
	    slaveId: number;
	    function: number;
	    address: number;
	    values: number[];
	
	    static createFrom(source: any = {}) {
	        return new WriteRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.connectionId = source["connectionId"];
	        this.slaveId = source["slaveId"];
	        this.function = source["function"];
	        this.address = source["address"];
	        this.values = source["values"];
	    }
	}

}

export namespace main {
	
	export class AppInfo {
	    version: string;
	    commit: string;
	    os: string;
	    arch: string;
	    repo: string;
	    canInstall: boolean;
	
	    static createFrom(source: any = {}) {
	        return new AppInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.version = source["version"];
	        this.commit = source["commit"];
	        this.os = source["os"];
	        this.arch = source["arch"];
	        this.repo = source["repo"];
	        this.canInstall = source["canInstall"];
	    }
	}
	export class CounterValue {
	    sub: number;
	    name: string;
	    value: number;
	    error: string;
	
	    static createFrom(source: any = {}) {
	        return new CounterValue(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.sub = source["sub"];
	        this.name = source["name"];
	        this.value = source["value"];
	        this.error = source["error"];
	    }
	}
	export class DataSettings {
	    folder: string;
	    defaultFolder: string;
	    retentionDays: number;
	    maxStorageMB: number;
	    usedBytes: number;
	    error: string;
	
	    static createFrom(source: any = {}) {
	        return new DataSettings(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.folder = source["folder"];
	        this.defaultFolder = source["defaultFolder"];
	        this.retentionDays = source["retentionDays"];
	        this.maxStorageMB = source["maxStorageMB"];
	        this.usedBytes = source["usedBytes"];
	        this.error = source["error"];
	    }
	}
	export class EchoResult {
	    sent: number;
	    ok: number;
	    minMs: number;
	    avgMs: number;
	    maxMs: number;
	    errors: string[];
	
	    static createFrom(source: any = {}) {
	        return new EchoResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.sent = source["sent"];
	        this.ok = source["ok"];
	        this.minMs = source["minMs"];
	        this.avgMs = source["avgMs"];
	        this.maxMs = source["maxMs"];
	        this.errors = source["errors"];
	    }
	}
	export class ExportResult {
	    path: string;
	    rows: number;
	
	    static createFrom(source: any = {}) {
	        return new ExportResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.rows = source["rows"];
	    }
	}
	export class ImportPreview {
	    path: string;
	    fileName: string;
	    rows: string[][];
	    totalRows: number;
	    options: profile.ImportOptions;
	    fields: string[];
	
	    static createFrom(source: any = {}) {
	        return new ImportPreview(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.fileName = source["fileName"];
	        this.rows = source["rows"];
	        this.totalRows = source["totalRows"];
	        this.options = this.convertValues(source["options"], profile.ImportOptions);
	        this.fields = source["fields"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class ImportResult {
	    profile: profile.Profile;
	    warnings: string[];
	
	    static createFrom(source: any = {}) {
	        return new ImportResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.profile = this.convertValues(source["profile"], profile.Profile);
	        this.warnings = source["warnings"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class MissingPort {
	    connectionId: string;
	    name: string;
	    port: string;
	
	    static createFrom(source: any = {}) {
	        return new MissingPort(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.connectionId = source["connectionId"];
	        this.name = source["name"];
	        this.port = source["port"];
	    }
	}
	export class WorkspaceState {
	    path: string;
	    name: string;
	    dirty: boolean;
	    recent: string[];
	
	    static createFrom(source: any = {}) {
	        return new WorkspaceState(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.name = source["name"];
	        this.dirty = source["dirty"];
	        this.recent = source["recent"];
	    }
	}
	export class LoadResult {
	    state: WorkspaceState;
	    layout: workspace.Layout;
	    charts: any;
	    sequences: any;
	    warnings: string[];
	    missingPorts: MissingPort[];
	    ports: string[];
	
	    static createFrom(source: any = {}) {
	        return new LoadResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.state = this.convertValues(source["state"], WorkspaceState);
	        this.layout = this.convertValues(source["layout"], workspace.Layout);
	        this.charts = source["charts"];
	        this.sequences = source["sequences"];
	        this.warnings = source["warnings"];
	        this.missingPorts = this.convertValues(source["missingPorts"], MissingPort);
	        this.ports = source["ports"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	
	export class ProfileFolders {
	    user: string;
	    shared: string;
	
	    static createFrom(source: any = {}) {
	        return new ProfileFolders(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.user = source["user"];
	        this.shared = source["shared"];
	    }
	}
	export class SeriesRef {
	    connection: string;
	    slaveId: number;
	    function: number;
	    address: number;
	    format: string;
	
	    static createFrom(source: any = {}) {
	        return new SeriesRef(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.connection = source["connection"];
	        this.slaveId = source["slaveId"];
	        this.function = source["function"];
	        this.address = source["address"];
	        this.format = source["format"];
	    }
	}
	export class UpdateState {
	    status: string;
	    release?: update.Release;
	    error: string;
	    checkedAt: number;
	    skipped: boolean;
	
	    static createFrom(source: any = {}) {
	        return new UpdateState(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.status = source["status"];
	        this.release = this.convertValues(source["release"], update.Release);
	        this.error = source["error"];
	        this.checkedAt = source["checkedAt"];
	        this.skipped = source["skipped"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}

}

export namespace modbus {
	
	export class DeviceObject {
	    id: number;
	    name: string;
	    value: string;
	
	    static createFrom(source: any = {}) {
	        return new DeviceObject(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.value = source["value"];
	    }
	}
	export class DeviceID {
	    conformity: number;
	    objects: DeviceObject[];
	
	    static createFrom(source: any = {}) {
	        return new DeviceID(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.conformity = source["conformity"];
	        this.objects = this.convertValues(source["objects"], DeviceObject);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}

}

export namespace profile {
	
	export class ImportOptions {
	    headerRow: number;
	    mapping: Record<string, number>;
	    addressMode: string;
	    defaultFunction: number;
	    byteOrder: string;
	
	    static createFrom(source: any = {}) {
	        return new ImportOptions(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.headerRow = source["headerRow"];
	        this.mapping = source["mapping"];
	        this.addressMode = source["addressMode"];
	        this.defaultFunction = source["defaultFunction"];
	        this.byteOrder = source["byteOrder"];
	    }
	}
	export class Register {
	    address: number;
	    function: number;
	    name: string;
	    format: string;
	    gain: number;
	    offset: number;
	    unit: string;
	    decimals: number;
	    note?: string;
	    alarm?: engine.AlarmLimits;
	
	    static createFrom(source: any = {}) {
	        return new Register(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.address = source["address"];
	        this.function = source["function"];
	        this.name = source["name"];
	        this.format = source["format"];
	        this.gain = source["gain"];
	        this.offset = source["offset"];
	        this.unit = source["unit"];
	        this.decimals = source["decimals"];
	        this.note = source["note"];
	        this.alarm = this.convertValues(source["alarm"], engine.AlarmLimits);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class Profile {
	    format: string;
	    version: number;
	    name: string;
	    manufacturer: string;
	    model: string;
	    description: string;
	    defaultSlaveId: number;
	    registers: Register[];
	
	    static createFrom(source: any = {}) {
	        return new Profile(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.format = source["format"];
	        this.version = source["version"];
	        this.name = source["name"];
	        this.manufacturer = source["manufacturer"];
	        this.model = source["model"];
	        this.description = source["description"];
	        this.defaultSlaveId = source["defaultSlaveId"];
	        this.registers = this.convertValues(source["registers"], Register);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	
	export class Summary {
	    id: string;
	    source: string;
	    name: string;
	    manufacturer: string;
	    model: string;
	    registers: number;
	    path: string;
	    error?: string;
	
	    static createFrom(source: any = {}) {
	        return new Summary(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.source = source["source"];
	        this.name = source["name"];
	        this.manufacturer = source["manufacturer"];
	        this.model = source["model"];
	        this.registers = source["registers"];
	        this.path = source["path"];
	        this.error = source["error"];
	    }
	}

}

export namespace recorder {
	
	export class Sample {
	    t: number;
	    v: number;
	
	    static createFrom(source: any = {}) {
	        return new Sample(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.t = source["t"];
	        this.v = source["v"];
	    }
	}
	export class SeriesInfo {
	    id: string;
	    connection: string;
	    slaveId: number;
	    function: number;
	    address: number;
	    format: string;
	    name: string;
	    unit: string;
	    window: string;
	    bytes: number;
	    samples: number;
	    first: number;
	    last: number;
	
	    static createFrom(source: any = {}) {
	        return new SeriesInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.connection = source["connection"];
	        this.slaveId = source["slaveId"];
	        this.function = source["function"];
	        this.address = source["address"];
	        this.format = source["format"];
	        this.name = source["name"];
	        this.unit = source["unit"];
	        this.window = source["window"];
	        this.bytes = source["bytes"];
	        this.samples = source["samples"];
	        this.first = source["first"];
	        this.last = source["last"];
	    }
	}

}

export namespace settings {
	
	export class Settings {
	    updateChannel: string;
	    autoCheckUpdates: boolean;
	    skippedVersion: string;
	    lastUpdateCheckAt: number;
	    recentWorkspaces: string[];
	    sharedProfileDir: string;
	    recordingsDir: string;
	    retentionDays: number;
	    maxStorageMB: number;
	
	    static createFrom(source: any = {}) {
	        return new Settings(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.updateChannel = source["updateChannel"];
	        this.autoCheckUpdates = source["autoCheckUpdates"];
	        this.skippedVersion = source["skippedVersion"];
	        this.lastUpdateCheckAt = source["lastUpdateCheckAt"];
	        this.recentWorkspaces = source["recentWorkspaces"];
	        this.sharedProfileDir = source["sharedProfileDir"];
	        this.recordingsDir = source["recordingsDir"];
	        this.retentionDays = source["retentionDays"];
	        this.maxStorageMB = source["maxStorageMB"];
	    }
	}

}

export namespace simulator {
	
	export class Config {
	    mode: string;
	    serialPort: string;
	    baudRate: number;
	    dataBits: number;
	    parity: string;
	    stopBits: number;
	    host: string;
	    port: number;
	    slaveId: number;
	    animate: boolean;
	    limits?: Record<string, string>;
	
	    static createFrom(source: any = {}) {
	        return new Config(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.mode = source["mode"];
	        this.serialPort = source["serialPort"];
	        this.baudRate = source["baudRate"];
	        this.dataBits = source["dataBits"];
	        this.parity = source["parity"];
	        this.stopBits = source["stopBits"];
	        this.host = source["host"];
	        this.port = source["port"];
	        this.slaveId = source["slaveId"];
	        this.animate = source["animate"];
	        this.limits = source["limits"];
	    }
	}
	export class Status {
	    running: boolean;
	    config: Config;
	    requests: number;
	    clients: number;
	    error: string;
	
	    static createFrom(source: any = {}) {
	        return new Status(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.running = source["running"];
	        this.config = this.convertValues(source["config"], Config);
	        this.requests = source["requests"];
	        this.clients = source["clients"];
	        this.error = source["error"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}

}

export namespace update {
	
	export class Asset {
	    name: string;
	    url: string;
	    size: number;
	
	    static createFrom(source: any = {}) {
	        return new Asset(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.url = source["url"];
	        this.size = source["size"];
	    }
	}
	export class Release {
	    version: string;
	    tag: string;
	    name: string;
	    notes: string;
	    pageUrl: string;
	    prerelease: boolean;
	    publishedAt: string;
	    asset: Asset;
	    installer?: Asset;
	
	    static createFrom(source: any = {}) {
	        return new Release(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.version = source["version"];
	        this.tag = source["tag"];
	        this.name = source["name"];
	        this.notes = source["notes"];
	        this.pageUrl = source["pageUrl"];
	        this.prerelease = source["prerelease"];
	        this.publishedAt = source["publishedAt"];
	        this.asset = this.convertValues(source["asset"], Asset);
	        this.installer = this.convertValues(source["installer"], Asset);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}

}

export namespace workspace {
	
	export class Layout {
	    view: string;
	    activeId: string;
	    showTraffic: boolean;
	    showSimulator: boolean;
	
	    static createFrom(source: any = {}) {
	        return new Layout(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.view = source["view"];
	        this.activeId = source["activeId"];
	        this.showTraffic = source["showTraffic"];
	        this.showSimulator = source["showSimulator"];
	    }
	}

}

