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

