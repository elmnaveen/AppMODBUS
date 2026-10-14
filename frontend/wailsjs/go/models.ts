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

