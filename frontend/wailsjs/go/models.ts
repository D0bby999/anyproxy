export namespace driver {
	
	export class EngineStatus {
	    state: string;
	    enabled: boolean;
	    backend: string;
	    extensionInstalled: boolean;
	    dnsProxyActive: boolean;
	    activeProxies: number;
	    activeRules: number;
	    activeFlows: number;
	    message?: string;
	
	    static createFrom(source: any = {}) {
	        return new EngineStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.state = source["state"];
	        this.enabled = source["enabled"];
	        this.backend = source["backend"];
	        this.extensionInstalled = source["extensionInstalled"];
	        this.dnsProxyActive = source["dnsProxyActive"];
	        this.activeProxies = source["activeProxies"];
	        this.activeRules = source["activeRules"];
	        this.activeFlows = source["activeFlows"];
	        this.message = source["message"];
	    }
	}

}

export namespace flows {
	
	export class ActivityLogEntry {
	    id: number;
	    timestamp: string;
	    level: string;
	    message: string;
	
	    static createFrom(source: any = {}) {
	        return new ActivityLogEntry(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.timestamp = source["timestamp"];
	        this.level = source["level"];
	        this.message = source["message"];
	    }
	}
	export class ConnectionLogEntry {
	    id: number;
	    timestamp: string;
	    pid: number;
	    processName: string;
	    destIp: string;
	    host: string;
	    port: number;
	    protocol: string;
	    action: string;
	    proxyName: string;
	    ruleId: string;
	    count: number;
	
	    static createFrom(source: any = {}) {
	        return new ConnectionLogEntry(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.timestamp = source["timestamp"];
	        this.pid = source["pid"];
	        this.processName = source["processName"];
	        this.destIp = source["destIp"];
	        this.host = source["host"];
	        this.port = source["port"];
	        this.protocol = source["protocol"];
	        this.action = source["action"];
	        this.proxyName = source["proxyName"];
	        this.ruleId = source["ruleId"];
	        this.count = source["count"];
	    }
	}
	export class Flow {
	    id: string;
	    pid: number;
	    processName: string;
	    destIp: string;
	    host: string;
	    port: number;
	    isUdp: boolean;
	    isIpv6: boolean;
	    proxyId: string;
	    proxyLabel: string;
	    ruleId: string;
	    ageMs: number;
	    sent: number;
	    received: number;
	    closed: boolean;
	    startedAt: number;
	    closedAt?: number;
	
	    static createFrom(source: any = {}) {
	        return new Flow(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.pid = source["pid"];
	        this.processName = source["processName"];
	        this.destIp = source["destIp"];
	        this.host = source["host"];
	        this.port = source["port"];
	        this.isUdp = source["isUdp"];
	        this.isIpv6 = source["isIpv6"];
	        this.proxyId = source["proxyId"];
	        this.proxyLabel = source["proxyLabel"];
	        this.ruleId = source["ruleId"];
	        this.ageMs = source["ageMs"];
	        this.sent = source["sent"];
	        this.received = source["received"];
	        this.closed = source["closed"];
	        this.startedAt = source["startedAt"];
	        this.closedAt = source["closedAt"];
	    }
	}

}

export namespace netproxy {
	
	export class CheckStep {
	    step: number;
	    name: string;
	    status: string;
	    latencyMs?: number;
	    message: string;
	
	    static createFrom(source: any = {}) {
	        return new CheckStep(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.step = source["step"];
	        this.name = source["name"];
	        this.status = source["status"];
	        this.latencyMs = source["latencyMs"];
	        this.message = source["message"];
	    }
	}
	export class CheckReport {
	    proxyId: string;
	    proxyName: string;
	    proxyType: string;
	    proxyAddr: string;
	    target: string;
	    success: boolean;
	    udpOk: boolean;
	    minMs: number;
	    avgMs: number;
	    maxMs: number;
	    summary: string;
	    steps: CheckStep[];
	    finishedAt: string;
	
	    static createFrom(source: any = {}) {
	        return new CheckReport(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.proxyId = source["proxyId"];
	        this.proxyName = source["proxyName"];
	        this.proxyType = source["proxyType"];
	        this.proxyAddr = source["proxyAddr"];
	        this.target = source["target"];
	        this.success = source["success"];
	        this.udpOk = source["udpOk"];
	        this.minMs = source["minMs"];
	        this.avgMs = source["avgMs"];
	        this.maxMs = source["maxMs"];
	        this.summary = source["summary"];
	        this.steps = this.convertValues(source["steps"], CheckStep);
	        this.finishedAt = source["finishedAt"];
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

export namespace pbprofile {
	
	export class LogFilter {
	    id: string;
	    mode: string;
	    process: string;
	    host: string;
	    port: string;
	    protocol: string;
	    action: string;
	
	    static createFrom(source: any = {}) {
	        return new LogFilter(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.mode = source["mode"];
	        this.process = source["process"];
	        this.host = source["host"];
	        this.port = source["port"];
	        this.protocol = source["protocol"];
	        this.action = source["action"];
	    }
	}
	export class ProxyRule {
	    id: string;
	    name: string;
	    processNames: string;
	    targetHosts: string;
	    targetDomains?: string;
	    targetPorts: string;
	    protocol: string;
	    action: string;
	    proxyConfigId?: string;
	    enabled: boolean;
	    excludeProcess?: boolean;
	    localhostViaProxy: boolean;
	
	    static createFrom(source: any = {}) {
	        return new ProxyRule(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.processNames = source["processNames"];
	        this.targetHosts = source["targetHosts"];
	        this.targetDomains = source["targetDomains"];
	        this.targetPorts = source["targetPorts"];
	        this.protocol = source["protocol"];
	        this.action = source["action"];
	        this.proxyConfigId = source["proxyConfigId"];
	        this.enabled = source["enabled"];
	        this.excludeProcess = source["excludeProcess"];
	        this.localhostViaProxy = source["localhostViaProxy"];
	    }
	}
	export class ProxyConfig {
	    id: string;
	    legacyId?: number;
	    name: string;
	    type: string;
	    host: string;
	    port: number;
	    username?: string;
	    password?: string;
	    sendDomainToProxy: boolean;
	
	    static createFrom(source: any = {}) {
	        return new ProxyConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.legacyId = source["legacyId"];
	        this.name = source["name"];
	        this.type = source["type"];
	        this.host = source["host"];
	        this.port = source["port"];
	        this.username = source["username"];
	        this.password = source["password"];
	        this.sendDomainToProxy = source["sendDomainToProxy"];
	    }
	}
	export class Profile {
	    id: string;
	    name: string;
	    trafficLoggingEnabled: boolean;
	    dnsViaProxy: boolean;
	    logEveryConnection: boolean;
	    autoClearConnectionLogs: boolean;
	    defaultProxyId?: string;
	    fontZoom: number;
	    closeToMenuBar: boolean;
	    runAtStartup: boolean;
	    proxies: ProxyConfig[];
	    rules: ProxyRule[];
	    logFilters?: LogFilter[];
	
	    static createFrom(source: any = {}) {
	        return new Profile(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.trafficLoggingEnabled = source["trafficLoggingEnabled"];
	        this.dnsViaProxy = source["dnsViaProxy"];
	        this.logEveryConnection = source["logEveryConnection"];
	        this.autoClearConnectionLogs = source["autoClearConnectionLogs"];
	        this.defaultProxyId = source["defaultProxyId"];
	        this.fontZoom = source["fontZoom"];
	        this.closeToMenuBar = source["closeToMenuBar"];
	        this.runAtStartup = source["runAtStartup"];
	        this.proxies = this.convertValues(source["proxies"], ProxyConfig);
	        this.rules = this.convertValues(source["rules"], ProxyRule);
	        this.logFilters = this.convertValues(source["logFilters"], LogFilter);
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

export namespace service {
	
	export class ProfileListSummary {
	    names: string[];
	    active: string;
	
	    static createFrom(source: any = {}) {
	        return new ProfileListSummary(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.names = source["names"];
	        this.active = source["active"];
	    }
	}

}

export namespace update {
	
	export class ReleaseInfo {
	    currentVersion: string;
	    latestVersion: string;
	    isUpdateAvailable: boolean;
	    releaseDate?: string;
	    releaseNotes?: string;
	    downloadUrl?: string;
	    error?: string;
	
	    static createFrom(source: any = {}) {
	        return new ReleaseInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.currentVersion = source["currentVersion"];
	        this.latestVersion = source["latestVersion"];
	        this.isUpdateAvailable = source["isUpdateAvailable"];
	        this.releaseDate = source["releaseDate"];
	        this.releaseNotes = source["releaseNotes"];
	        this.downloadUrl = source["downloadUrl"];
	        this.error = source["error"];
	    }
	}

}

