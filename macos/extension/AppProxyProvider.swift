import NetworkExtension
import Foundation

enum RuleProtocol: String, Codable {
    case tcp = "TCP"
    case udp = "UDP"
    case both = "BOTH"
}

struct ProxyRule: Codable {
    var ruleId: UInt32
    let processNames: String
    let targetHosts: String
    let targetPorts: String
    let ruleProtocol: RuleProtocol
    let action: String  // "DIRECT", "BLOCK", or a proxy config UUID
    var enabled: Bool
    var excludeProcess: Bool
    var localhostViaProxy: Bool

    enum CodingKeys: String, CodingKey {
        case ruleId
        case processNames
        case targetHosts
        case targetPorts
        case ruleProtocol
        case action = "ruleAction"
        case enabled
        case excludeProcess
        case localhostViaProxy
    }

    init(from decoder: Decoder) throws {
        let container = try decoder.container(keyedBy: CodingKeys.self)
        self.ruleId = try container.decodeIfPresent(UInt32.self, forKey: .ruleId) ?? 0
        self.processNames = try container.decode(String.self, forKey: .processNames)
        self.targetHosts = try container.decode(String.self, forKey: .targetHosts)
        self.targetPorts = try container.decode(String.self, forKey: .targetPorts)
        self.ruleProtocol = try container.decode(RuleProtocol.self, forKey: .ruleProtocol)
        self.action = try container.decode(String.self, forKey: .action)
        self.enabled = try container.decodeIfPresent(Bool.self, forKey: .enabled) ?? true
        self.excludeProcess = try container.decodeIfPresent(Bool.self, forKey: .excludeProcess) ?? false
        self.localhostViaProxy = try container.decodeIfPresent(Bool.self, forKey: .localhostViaProxy) ?? false
    }

    init(ruleId: UInt32, processNames: String, targetHosts: String, targetPorts: String, ruleProtocol: RuleProtocol, action: String, enabled: Bool, excludeProcess: Bool = false, localhostViaProxy: Bool = false) {
        self.ruleId = ruleId
        self.processNames = processNames
        self.targetHosts = targetHosts
        self.targetPorts = targetPorts
        self.ruleProtocol = ruleProtocol
        self.action = action
        self.enabled = enabled
        self.excludeProcess = excludeProcess
        self.localhostViaProxy = localhostViaProxy
    }
    
    func matchesProcess(bundleId: String, processName: String?) -> Bool {
        if Self.matchProcessList(processNames, processPath: bundleId) {
            return true
        }
        
        if let procName = processName {
            if Self.matchProcessList(processNames, processPath: procName) {
                return true
            }
        }
        
        return false
    }
    
    func matchesIP(_ ipString: String) -> Bool {
        return Self.matchIPList(targetHosts, ipString: ipString)
    }

    // matches targetHosts against the destination ip and any domains that
    // resolved to it (from the dns proxy). a pattern can be an ip/range or a
    // domain like *.github.com
    func matchesHost(ip: String, domains: [String]) -> Bool {
        if targetHosts.isEmpty || targetHosts == "*" { return true }
        for raw in targetHosts.components(separatedBy: CharacterSet(charactersIn: ",;")) {
            let pattern = raw.trimmingCharacters(in: .whitespacesAndNewlines)
            if pattern.isEmpty { continue }
            if Self.matchIPPattern(pattern, ipString: ip) { return true }

            let lowered = pattern.lowercased()
            // *.example.com also covers the apex example.com, which is what users expect
            let apex = lowered.hasPrefix("*.") ? String(lowered.dropFirst(2)) : nil
            for domain in domains {
                if Self.globMatch(lowered, domain) { return true }
                if let apex = apex, domain == apex { return true }
            }
        }
        return false
    }


    func matchesPort(_ port: UInt16) -> Bool {
        return Self.matchPortList(targetPorts, port: port)
    }
    
    private static func matchProcessList(_ processList: String, processPath: String) -> Bool {
        if processList.isEmpty || processList == "*" {
            return true
        }
        
        let filename = (processPath as NSString).lastPathComponent
        let patterns = processList.components(separatedBy: CharacterSet(charactersIn: ",;"))
        
        for pattern in patterns {
            let trimmed = pattern.trimmingCharacters(in: .whitespacesAndNewlines)
            if matchProcessPattern(trimmed, processPath: processPath, filename: filename) {
                return true
            }
        }
        return false
    }
    
    private static func matchProcessPattern(_ pattern: String, processPath: String, filename: String) -> Bool {
        if pattern.isEmpty || pattern == "*" {
            return true
        }

        let isFullPathPattern = pattern.contains("/") || pattern.contains("\\")
        let matchTarget = isFullPathPattern ? processPath : filename
        return globMatch(pattern.lowercased(), matchTarget.lowercased())
    }

    // wildcard match supporting any number of "*" and "?", e.g. *chrome*, com.*.browser, api?.example.org
    private static func globMatch(_ pattern: String, _ text: String) -> Bool {
        return globMatch(Array(pattern), Array(text))
    }

    fileprivate static func globMatch(_ p: [Character], _ t: [Character]) -> Bool {
        var pi = 0, ti = 0, star = -1, mark = 0
        while ti < t.count {
            if pi < p.count, (p[pi] == t[ti] || p[pi] == "?") {
                pi += 1; ti += 1
            } else if pi < p.count, p[pi] == "*" {
                star = pi; mark = ti; pi += 1
            } else if star != -1 {
                pi = star + 1; mark += 1; ti = mark
            } else {
                return false
            }
        }
        while pi < p.count, p[pi] == "*" { pi += 1 }
        return pi == p.count
    }
    
    private static func matchIPList(_ ipList: String, ipString: String) -> Bool {
        if ipList.isEmpty || ipList == "*" {
            return true
        }
        
        let patterns = ipList.components(separatedBy: CharacterSet(charactersIn: ",;"))
        for pattern in patterns {
            let trimmed = pattern.trimmingCharacters(in: .whitespacesAndNewlines)
            if matchIPPattern(trimmed, ipString: ipString) {
                return true
            }
        }
        return false
    }
    
    // raw bytes of a v4 (4) or v6 (16) address, nil if not an ip literal
    private static func ipToBytes(_ ipString: String) -> [UInt8]? {
        if let v4 = IPv4Address(ipString) { return Array(v4.rawValue) }
        if let v6 = IPv6Address(ipString) { return Array(v6.rawValue) }
        return nil
    }

    private static func compareBytes(_ a: [UInt8], _ b: [UInt8]) -> Int {
        for i in 0..<min(a.count, b.count) where a[i] != b[i] {
            return a[i] < b[i] ? -1 : 1
        }
        return 0
    }

    private static func matchCIDR(_ pattern: String, ipString: String) -> Bool {
        guard let slash = pattern.firstIndex(of: "/") else { return false }
        let netStr = String(pattern[..<slash]).trimmingCharacters(in: .whitespaces)
        let pfxStr = String(pattern[pattern.index(after: slash)...]).trimmingCharacters(in: .whitespaces)
        guard let prefix = Int(pfxStr),
              let netBytes = ipToBytes(netStr),
              let targetBytes = ipToBytes(ipString),
              netBytes.count == targetBytes.count else {
            return false
        }
        let maxBits = netBytes.count * 8
        guard prefix >= 0 && prefix <= maxBits else { return false }
        let fullBytes = prefix / 8
        let remBits = prefix % 8
        for i in 0..<fullBytes where netBytes[i] != targetBytes[i] {
            return false
        }
        if remBits > 0 {
            let mask = UInt8(0xFF << (8 - remBits))
            if (netBytes[fullBytes] & mask) != (targetBytes[fullBytes] & mask) {
                return false
            }
        }
        return true
    }

    // full 8 hextet form of a v6 address, e.g. ["2001","db8","0",...]
    private static func expandIPv6(_ ipString: String) -> [String]? {
        guard let v6 = IPv6Address(ipString) else { return nil }
        let bytes = Array(v6.rawValue)
        var groups: [String] = []
        for i in stride(from: 0, to: 16, by: 2) {
            let value = (UInt16(bytes[i]) << 8) | UInt16(bytes[i + 1])
            groups.append(String(format: "%x", value))
        }
        return groups
    }

    fileprivate static func matchIPPattern(_ pattern: String, ipString: String) -> Bool {
        if pattern.isEmpty || pattern == "*" {
            return true
        }

        // CIDR notation, e.g. 10.0.0.0/8 or 2001:db8::/32
        if pattern.contains("/") {
            return matchCIDR(pattern, ipString: ipString)
        }

        // full IP range, works for v4 and v6, e.g. 10.0.0.1-10.0.0.254 or fe80::1-fe80::ff
        if pattern.contains("-") {
            let parts = pattern.components(separatedBy: "-")
            if parts.count == 2,
               let lo = ipToBytes(parts[0].trimmingCharacters(in: .whitespaces)),
               let hi = ipToBytes(parts[1].trimmingCharacters(in: .whitespaces)),
               let target = ipToBytes(ipString),
               lo.count == hi.count, target.count == lo.count {
                return compareBytes(target, lo) >= 0 && compareBytes(target, hi) <= 0
            }
        }

        // v6 pattern (contains a colon)
        if pattern.contains(":") {
            return matchIPv6Pattern(pattern, ipString: ipString)
        }

        // v4 wildcard / per-octet range, e.g. 192.168.1.* or 10.1-5.*.*
        let patternOctets = pattern.components(separatedBy: ".")
        let ipOctets = ipString.components(separatedBy: ".")
        if patternOctets.count != 4 || ipOctets.count != 4 {
            return false
        }
        for i in 0..<4 {
            let pOct = patternOctets[i]
            if pOct == "*" { continue }
            guard let ipVal = UInt8(ipOctets[i]) else { return false }
            if let dash = pOct.firstIndex(of: "-") {
                guard let lo = UInt8(pOct[..<dash]),
                      let hi = UInt8(pOct[pOct.index(after: dash)...]),
                      lo <= hi else { return false }
                if ipVal < lo || ipVal > hi { return false }
            } else {
                guard let exact = UInt8(pOct), ipVal == exact else { return false }
            }
        }
        return true
    }

    private static func matchIPv6Pattern(_ pattern: String, ipString: String) -> Bool {
        // prefix wildcard on leading hextets, e.g. 2001:db8:* (no :: compression)
        if pattern.hasSuffix("*") {
            guard let target = expandIPv6(ipString) else { return false }
            var groups = pattern.lowercased().dropLast()
                .split(separator: ":", omittingEmptySubsequences: false).map(String.init)
            if groups.last == "" { groups.removeLast() }
            for (i, g) in groups.enumerated() {
                guard i < 8, !g.isEmpty, let value = UInt16(g, radix: 16) else { return false }
                if String(format: "%x", value) != target[i] { return false }
            }
            return true
        }

        // exact match, compared on normalized bytes so ::1 == 0:0:...:1
        guard let p = ipToBytes(pattern), let t = ipToBytes(ipString) else { return false }
        return p == t
    }
    
    private static func matchPortList(_ portList: String, port: UInt16) -> Bool {
        if portList.isEmpty || portList == "*" {
            return true
        }
        
        let patterns = portList.components(separatedBy: CharacterSet(charactersIn: ",;"))
        for pattern in patterns {
            let trimmed = pattern.trimmingCharacters(in: .whitespacesAndNewlines)
            if matchPortPattern(trimmed, port: port) {
                return true
            }
        }
        return false
    }
    
    private static func matchPortPattern(_ pattern: String, port: UInt16) -> Bool {
        if pattern.isEmpty || pattern == "*" {
            return true
        }
        
        if let dashIndex = pattern.firstIndex(of: "-") {
            let startStr = String(pattern[..<dashIndex])
            let endStr = String(pattern[pattern.index(after: dashIndex)...])
            
            if let start = UInt16(startStr), let end = UInt16(endStr) {
                return port >= start && port <= end
            }
            return false
        }
        
        if let patternPort = UInt16(pattern) {
            return port == patternPort
        }
        return false
    }
}

// Pre-parsed form of a rule. Rules are matched on every new flow, so the string
// splitting, lowercasing and number parsing happens once when the rule is added
// instead of once per rule per connection.
extension ProxyRule {
    // lowercased forms of the flow's process, built once per flow
    struct FlowProcess {
        let candidates: [(path: [Character], file: [Character])]

        // the names a rule can match: the bundle identifier, and the program itself. when the full path of
        // the program is known a rule written with a path (/opt/tool/bin/worker, /Applications/Foo.app/*)
        // is compared with it, and a rule written with a file name with its last part. without the path
        // only the file name is known and a rule with a path cannot match (issue #288).
        init(bundleId: String, processName: String?, executablePath: String? = nil, ancestorPaths: [String] = []) {
            var list = [bundleId]
            if let executablePath = executablePath {
                list.append(executablePath)
            } else if let processName = processName {
                list.append(processName)
            }
            list.append(contentsOf: ancestorPaths)
            candidates = list.map {
                (Array($0.lowercased()), Array((($0 as NSString).lastPathComponent).lowercased()))
            }
        }
    }

    struct Matcher {
        let processAll: Bool
        let processPatterns: [(chars: [Character], fullPath: Bool)]
        let hostAll: Bool
        let hostPatterns: [(raw: String, glob: [Character], apex: String?)]
        let portAll: Bool
        let portRanges: [ClosedRange<UInt16>]

        init(_ rule: ProxyRule) {
            func isAll(_ s: String) -> Bool { s.isEmpty || s == "*" }

            // empty segments are skipped, "Safari;" used to match every process
            // because the empty pattern after the ; counted as a wildcard
            var all = isAll(rule.processNames)
            var procs: [(chars: [Character], fullPath: Bool)] = []
            if !all {
                for raw in rule.processNames.components(separatedBy: CharacterSet(charactersIn: ",;")) {
                    let t = raw.trimmingCharacters(in: .whitespacesAndNewlines)
                    if t.isEmpty { continue }
                    if t == "*" { all = true; break }
                    procs.append((Array(t.lowercased()), t.contains("/") || t.contains("\\")))
                }
            }
            processAll = all
            processPatterns = procs

            var hostsAll = isAll(rule.targetHosts)
            var hosts: [(raw: String, glob: [Character], apex: String?)] = []
            if !hostsAll {
                for raw in rule.targetHosts.components(separatedBy: CharacterSet(charactersIn: ",;")) {
                    let t = raw.trimmingCharacters(in: .whitespacesAndNewlines)
                    if t.isEmpty { continue }
                    if t == "*" { hostsAll = true; break }
                    let lowered = t.lowercased()
                    let apex = lowered.hasPrefix("*.") ? String(lowered.dropFirst(2)) : nil
                    hosts.append((t, Array(lowered), apex))
                }
            }
            hostAll = hostsAll
            hostPatterns = hosts

            var portsAll = isAll(rule.targetPorts)
            var ranges: [ClosedRange<UInt16>] = []
            if !portsAll {
                for raw in rule.targetPorts.components(separatedBy: CharacterSet(charactersIn: ",;")) {
                    let t = raw.trimmingCharacters(in: .whitespacesAndNewlines)
                    if t.isEmpty { continue }
                    if t == "*" { portsAll = true; break }
                    if let dash = t.firstIndex(of: "-") {
                        if let lo = UInt16(t[..<dash]), let hi = UInt16(t[t.index(after: dash)...]), lo <= hi {
                            ranges.append(lo...hi)
                        }
                    } else if let p = UInt16(t) {
                        ranges.append(p...p)
                    }
                }
            }
            portAll = portsAll
            portRanges = ranges
        }

        var hasHostFilter: Bool { !hostAll }
        var hasPortFilter: Bool { !portAll }

        func matchesProcess(_ flow: FlowProcess) -> Bool {
            if processAll { return true }
            for cand in flow.candidates {
                for p in processPatterns {
                    if ProxyRule.globMatch(p.chars, p.fullPath ? cand.path : cand.file) { return true }
                }
            }
            return false
        }

        func matchesHost(ip: String, domains: [(name: String, chars: [Character])]) -> Bool {
            if hostAll { return true }
            for p in hostPatterns {
                if ProxyRule.matchIPPattern(p.raw, ipString: ip) { return true }
                for d in domains {
                    if ProxyRule.globMatch(p.glob, d.chars) { return true }
                    if let apex = p.apex, d.name == apex { return true }
                }
            }
            return false
        }

        func matchesPort(_ port: UInt16) -> Bool {
            if portAll { return true }
            for r in portRanges where r.contains(port) { return true }
            return false
        }
    }
}

class AppProxyProvider: NETransparentProxyProvider {
    
    // one log entry, kept as an enum so we don't allocate a dictionary per line,
    // the dict is only built when the gui drains a batch
    private enum LogEntry {
        case connection(proto: String, pid: Int, process: String, destination: String, port: String, proxy: String)
        case activity(timestamp: String, level: String, message: String)

        func toDict() -> [String: String] {
            switch self {
            case let .connection(proto, pid, process, destination, port, proxy):
                return ["type": "connection", "protocol": proto, "pid": String(pid), "process": process,
                        "destination": destination, "port": port, "proxy": proxy]
            case let .activity(timestamp, level, message):
                return ["type": "activity", "timestamp": timestamp, "level": level, "message": message]
            }
        }
    }

    // circular buffer for logs, avoids shifting the whole array on every pop
    private static let logCapacity = 500
    private var logBuffer = [LogEntry?](repeating: nil, count: AppProxyProvider.logCapacity)
    private var logHead = 0
    private var logTail = 0
    private var logCount = 0
    private let logQueueLock = NSLock()
    private let dateFormatter: ISO8601DateFormatter = ISO8601DateFormatter()
    
    // cache by pid + pid version so we don't call proc_pidpath on every
    // connection. the version changes when a pid is reused, so a new process
    // never inherits the previous owner's name
    private struct ProcessIdentity {
        let pid: Int
        let name: String           // file name of the executable, what the logs show
        let path: String           // full path of the executable, what a rule with a path is compared with
        let ancestorPaths: [String] // parent .app / executable paths so child helpers & CLI tools inherit app rules
    }
    private var pidCache: [UInt64: ProcessIdentity] = [:]
    private let pidCacheLock = NSLock()
    private static let pidCacheMaxSize = 256
    private static let maxAncestorDepth = 4

    private static func collectAncestorPaths(startingFrom pid: pid_t, ownPath: String) -> [String] {
        var result: [String] = []
        var curPid = pid
        var pathBuf = [Int8](repeating: 0, count: Int(MAXPATHLEN))
        for _ in 0..<maxAncestorDepth {
            var bsdInfo = proc_bsdinfo()
            let size = Int32(MemoryLayout<proc_bsdinfo>.stride)
            guard proc_pidinfo(curPid, PROC_PIDTBSDINFO, 0, &bsdInfo, size) == size else {
                break
            }
            let ppid = pid_t(bsdInfo.pbi_ppid)
            if ppid <= 1 || ppid == curPid {
                break
            }
            if proc_pidpath(ppid, &pathBuf, UInt32(MAXPATHLEN)) > 0 {
                let pPath = String(cString: pathBuf)
                if pPath != ownPath && !result.contains(pPath) {
                    result.append(pPath)
                }
            }
            curPid = ppid
        }
        return result
    }
    
    private func getProcessIdentity(from metaData: NEFlowMetaData) -> ProcessIdentity? {
        guard let auditTokenData = metaData.sourceAppAuditToken else {
            return nil
        }
        guard auditTokenData.count == MemoryLayout<audit_token_t>.size else {
            return nil
        }
        
        let (pid, pidVersion) = auditTokenData.withUnsafeBytes { ptr -> (pid_t, UInt32) in
            guard let baseAddress = ptr.baseAddress else { return (0, 0) }
            let token = baseAddress.assumingMemoryBound(to: UInt32.self)
            return (pid_t(token[5]), token[7])
        }
        
        guard pid > 0 else { return nil }
        let key = (UInt64(UInt32(pid)) << 32) | UInt64(pidVersion)
        
        pidCacheLock.lock()
        if let cached = pidCache[key] {
            pidCacheLock.unlock()
            return cached
        }
        pidCacheLock.unlock()

        var pathBuffer = [Int8](repeating: 0, count: Int(MAXPATHLEN))
        guard proc_pidpath(pid, &pathBuffer, UInt32(MAXPATHLEN)) > 0 else {
            return nil
        }
        
        let fullPath = String(cString: pathBuffer)
        let ancestors = Self.collectAncestorPaths(startingFrom: pid, ownPath: fullPath)
        let identity = ProcessIdentity(pid: Int(pid), name: (fullPath as NSString).lastPathComponent, path: fullPath, ancestorPaths: ancestors)
        
        // store in cache, evict everything if full - processes rarely hit this
        pidCacheLock.lock()
        if pidCache.count >= AppProxyProvider.pidCacheMaxSize {
            pidCache.removeAll(keepingCapacity: true)
        }
        pidCache[key] = identity
        pidCacheLock.unlock()
        
        return identity
    }
    
    // guarded by its own lock, the old OSAtomic compare-and-swap setter could
    // silently no-op if the stored value wasn't the exact expected one
    private var _trafficLoggingEnabled = true
    private let trafficLoggingLock = NSLock()
    private var trafficLoggingEnabled: Bool {
        get { trafficLoggingLock.lock(); defer { trafficLoggingLock.unlock() }; return _trafficLoggingEnabled }
        set { trafficLoggingLock.lock(); _trafficLoggingEnabled = newValue; trafficLoggingLock.unlock() }
    }
    
    private typealias CompiledRule = (rule: ProxyRule, matcher: ProxyRule.Matcher)
    private var rules: [CompiledRule] = []
    private let rulesLock = NSLock()
    private var nextRuleId: UInt32 = 1
    
    private struct StoredProxyConfig: Equatable {
        let type: String
        let host: String
        let port: Int
        let username: String?
        let password: String?
        let sendDomain: Bool   // hand the proxy the host name instead of the ip when it is known
    }
    private var storedProxyConfigs: [String: StoredProxyConfig] = [:]
    private var defaultProxyId: String = ""
    // stays false until setRules lands after startProxy (or an OS respawn), so
    // flows arriving before config is pushed fail closed instead of going direct
    private var configLoaded = false
    private let proxyLock = NSLock()
    // a config change only affects connections opened after it. http/2 and quic
    // sockets that are already established keep the previous proxy for as long
    // as the browser holds them (hours), so the switch looks like it did nothing.
    // set when a pushed config differs from the previous one and acted on once
    // the matching rule list has been applied, dropping the flows that are
    // still running on the old proxy. guarded by proxyLock.
    private var pendingFlowReset = false
    // same idea for the rule list: a rewritten rule routes existing connections
    // the old way until their sockets close, which looks like the edit did nothing
    private var lastRulesSignature = ""
    // tracks whether 127.0.0.0/8 and ::1 are currently included in tunnel settings
    private var localhostInterceptEnabled = false
    private var _engineEnabled = true
    private let engineEnabledLock = NSLock()
    private var engineEnabled: Bool {
        get { engineEnabledLock.lock(); defer { engineEnabledLock.unlock() }; return _engineEnabled }
        set { engineEnabledLock.lock(); _engineEnabled = newValue; engineEnabledLock.unlock() }
    }
    // problems that would repeat on every connection are reported once per proxy list
    private var warnedKeys = Set<String>()

    private func warnOnce(_ key: String, _ message: String) {
        proxyLock.lock()
        let isNew = warnedKeys.insert(key).inserted
        proxyLock.unlock()
        if isNew { log(message, level: "ERROR") }
    }
    
    private func log(_ message: String, level: String = "INFO") {
        appendLog(.activity(timestamp: dateFormatter.string(from: Date()), level: level, message: message))
    }

    // the extension's own life cycle: to the unified log, which survives the extension, and to the
    // activity log, which the app shows while it is connected
    private func diag(_ message: String, level: String = "INFO") {
        ProviderDiagnostics.note(message, error: level == "ERROR")
        log(message, level: level)
    }

    private func appendLog(_ entry: LogEntry) {
        logQueueLock.lock()
        logBuffer[logTail] = entry
        logTail = (logTail + 1) % AppProxyProvider.logCapacity
        if logCount < AppProxyProvider.logCapacity {
            logCount += 1
        } else {
            // buffer full, bump head to drop the oldest entry
            logHead = (logHead + 1) % AppProxyProvider.logCapacity
        }
        logQueueLock.unlock()
    }

    private func makeTunnelSettings(includeLocalhost: Bool) -> NETransparentProxyNetworkSettings {
        let settings = NETransparentProxyNetworkSettings(tunnelRemoteAddress: "127.0.0.1")
        let allTrafficRule = NENetworkRule(
            remoteNetwork: nil,
            remotePrefix: 0,
            localNetwork: nil,
            localPrefix: 0,
            protocol: .any,
            direction: .outbound
        )
        settings.includedNetworkRules = [allTrafficRule]
        var excluded: [NENetworkRule] = [
            NENetworkRule(remoteNetwork: NWHostEndpoint(hostname: "169.254.0.0", port: "0"), remotePrefix: 16, localNetwork: nil, localPrefix: 0, protocol: .any, direction: .outbound),
            NENetworkRule(remoteNetwork: NWHostEndpoint(hostname: "fe80::", port: "0"), remotePrefix: 10, localNetwork: nil, localPrefix: 0, protocol: .any, direction: .outbound),
            NENetworkRule(remoteNetwork: NWHostEndpoint(hostname: "224.0.0.0", port: "0"), remotePrefix: 4, localNetwork: nil, localPrefix: 0, protocol: .any, direction: .outbound),
            NENetworkRule(remoteNetwork: NWHostEndpoint(hostname: "ff00::", port: "0"), remotePrefix: 8, localNetwork: nil, localPrefix: 0, protocol: .any, direction: .outbound),
        ]
        if !includeLocalhost {
            excluded.insert(NENetworkRule(remoteNetwork: NWHostEndpoint(hostname: "::1", port: "0"), remotePrefix: 128, localNetwork: nil, localPrefix: 0, protocol: .any, direction: .outbound), at: 0)
            excluded.insert(NENetworkRule(remoteNetwork: NWHostEndpoint(hostname: "127.0.0.0", port: "0"), remotePrefix: 8, localNetwork: nil, localPrefix: 0, protocol: .any, direction: .outbound), at: 0)
        }
        settings.excludedNetworkRules = excluded
        return settings
    }

    override func startProxy(options: [String : Any]?, completionHandler: @escaping (Error?) -> Void) {
        let settings = makeTunnelSettings(includeLocalhost: false)

        startUDPSweepTimer()
        diag("Tunnel starting")

        self.setTunnelNetworkSettings(settings) { [weak self] error in
            if let error = error {
                self?.diag("Could not apply the tunnel network settings: \(error.localizedDescription)", level: "ERROR")
            } else {
                self?.diag("Tunnel started")
            }
            completionHandler(error)
        }
    }
    
    override func stopProxy(with reason: NEProviderStopReason, completionHandler: @escaping () -> Void) {
        diag("Tunnel stopping: \(ProviderDiagnostics.text(for: reason))")
        udpSweepTimer?.cancel()
        udpSweepTimer = nil
        sweepHandshakes(all: true)
        udpLock.lock()
        let all = Array(udpAssociations.values)
        udpAssociations.removeAll()
        udpLock.unlock()
        for a in all {
            a.release()
        }
        // nothing is proxied while stopped, so don't keep stale tcp references
        openTCPLock.lock()
        openTCPFlows.removeAll()
        openTCPLock.unlock()
        proxyLock.lock()
        pendingFlowReset = false
        proxyLock.unlock()
        completionHandler()
    }
    
    // one association per udp flow, holds both channels so we tear the whole thing
    // down together when any side goes away
    private final class UDPAssociation {
        let clientFlow: NEAppProxyUDPFlow
        let controlConnection: NWTCPConnection  // socks5 tcp control channel, keeps the association alive
        let udpSession: NWUDPSession            // relay channel to the socks server
        let displayName: String
        var loggedDestinations = Set<String>()  // dedupe connection logs, bounded
        var isTornDown = false
        private var lastActivity = Date()
        private let activityLock = NSLock()

        // per-packet work is the hot path for quic/video, so the socks5 header for
        // a destination and the endpoint for a relay source are built once and
        // reused. both are capped so they can't grow forever.
        struct HostPort: Hashable { let host: String; let port: UInt16 }
        var headerCache: [HostPort: Data] = [:]
        var endpointCache: [UInt64: (host: String, port: UInt16)] = [:]
        static let cacheLimit = 64

        func touch() {
            activityLock.lock(); lastActivity = Date(); activityLock.unlock()
        }

        func idleTime() -> TimeInterval {
            activityLock.lock(); defer { activityLock.unlock() }
            return Date().timeIntervalSince(lastActivity)
        }

        // NWUDPSession keeps its read handler after cancel(), and that handler
        // refers back to this association, so the whole graph (association,
        // session, tcp control connection, flow) used to stay alive after every
        // udp flow. swap the handler out first, then cancel.
        func release() {
            udpSession.setReadHandler({ _, _ in }, maxDatagrams: 1)
            controlConnection.cancel()
            udpSession.cancel()
        }

        init(clientFlow: NEAppProxyUDPFlow, controlConnection: NWTCPConnection, udpSession: NWUDPSession, displayName: String) {
            self.clientFlow = clientFlow
            self.controlConnection = controlConnection
            self.udpSession = udpSession
            self.displayName = displayName
        }
    }
    private var udpAssociations: [NEAppProxyUDPFlow: UDPAssociation] = [:]
    private let udpLock = NSLock()

    // udp has no close signal we can rely on (quic flows come and go), so idle
    // associations are reaped here, otherwise each one pins a flow, a tcp
    // control connection and a udp session for the life of the extension
    private static let udpIdleTimeout: TimeInterval = 120
    private var udpSweepTimer: DispatchSourceTimer?

    private func startUDPSweepTimer() {
        udpSweepTimer?.cancel()
        let timer = DispatchSource.makeTimerSource(queue: DispatchQueue.global(qos: .utility))
        timer.schedule(deadline: .now() + 5, repeating: 5, leeway: .seconds(1))
        timer.setEventHandler { [weak self] in
            guard let self = self else { return }
            self.sweepHandshakes()
            self.udpLock.lock()
            let stale = self.udpAssociations.filter { $0.value.idleTime() > AppProxyProvider.udpIdleTimeout }.map { $0.key }
            self.udpLock.unlock()
            for flow in stale { self.teardownUDP(flow) }
        }
        timer.resume()
        udpSweepTimer = timer
    }

    // tcp and udp proxy handshakes have no timeout of their own, so a proxy that
    // accepts the connection and then goes quiet would pin the flow forever.
    // entries are checked by the shared sweep timer, no per-flow timer is
    // scheduled (a timer closure would hold the flow for its full delay even
    // after the handshake succeeded, which adds up with quic churn)
    private var pendingHandshakes: [ObjectIdentifier: (deadline: Date, flow: NEAppProxyFlow, connection: NWTCPConnection)] = [:]
    private let handshakeLock = NSLock()
    private static let handshakeTimeout: TimeInterval = 20
    // grace between a client FIN and tearing the relay down, lets the upstream FIN flush
    private static let halfCloseTimeout: TimeInterval = 3

    private func armHandshakeTimeout(flow: NEAppProxyFlow, connection: NWTCPConnection) {
        let deadline = Date().addingTimeInterval(AppProxyProvider.handshakeTimeout)
        handshakeLock.lock()
        pendingHandshakes[ObjectIdentifier(connection)] = (deadline, flow, connection)
        handshakeLock.unlock()
    }

    // false means the sweep already timed it out and tore everything down
    private func completeHandshake(_ connection: NWTCPConnection) -> Bool {
        handshakeLock.lock()
        defer { handshakeLock.unlock() }
        return pendingHandshakes.removeValue(forKey: ObjectIdentifier(connection)) != nil
    }

    private func sweepHandshakes(now: Date = Date(), all: Bool = false) {
        handshakeLock.lock()
        let expired = pendingHandshakes.filter { all || $0.value.deadline <= now }
        for key in expired.keys { pendingHandshakes.removeValue(forKey: key) }
        handshakeLock.unlock()
        for (_, entry) in expired {
            unregisterLiveFlow(entry.flow)
            entry.connection.cancel()
            // a tcp flow must fail its connect (see FlowFailure), udp flows keep the clean close
            let reason: Error? = entry.flow is NEAppProxyTCPFlow ? FlowFailure.timedOut : nil
            if let tcp = entry.flow as? NEAppProxyTCPFlow {
                // the relay teardown may get here too, only one of them closes it
                guard claimTCPFlowClose(tcp) else { continue }
            }
            entry.flow.closeReadWithError(reason)
            entry.flow.closeWriteWithError(reason)
        }
    }

    private struct LiveFlowInfo {
        let id: String
        let pid: Int
        let processName: String
        var destIp: String
        var host: String
        var port: UInt16
        let isUdp: Bool
        let proxyLabel: String
        var sent: Int64
        var received: Int64
        let startedAt: Date
    }
    private var activeFlows: [ObjectIdentifier: LiveFlowInfo] = [:]
    private let activeFlowsLock = NSLock()
    private var nextFlowSeq: UInt64 = 1

    // the ui registry above keeps stats after a flow is gone, so it can't be used
    // to close anything. this one holds the live tcp flows that are currently
    // proxied, which is what a config change has to drop.
    private var openTCPFlows: [ObjectIdentifier: NEAppProxyTCPFlow] = [:]
    // a flow can be closed by the reset and again by its own relay teardown.
    // closing an NEAppProxyFlow twice is not safe, so the first path that gets
    // here claims it and the second one backs off.
    private var closedTCPFlows = Set<ObjectIdentifier>()
    private let openTCPLock = NSLock()

    private func trackTCPFlow(_ flow: NEAppProxyTCPFlow) {
        openTCPLock.lock()
        openTCPFlows[ObjectIdentifier(flow)] = flow
        openTCPLock.unlock()
    }

    // true when this call is the one that may close the flow
    private func claimTCPFlowClose(_ flow: NEAppProxyTCPFlow) -> Bool {
        let key = ObjectIdentifier(flow)
        openTCPLock.lock()
        defer { openTCPLock.unlock() }
        openTCPFlows.removeValue(forKey: key)
        if closedTCPFlows.contains(key) {
            return false
        }
        closedTCPFlows.insert(key)
        // bounded so the tombstones can't grow with flow churn
        if closedTCPFlows.count > 512 {
            closedTCPFlows.removeAll()
        }
        return true
    }

    // drops every flow that is still running on the old proxy list so the apps
    // open fresh connections and match against the new one
    private func resetOpenFlows(reason: String) {
        openTCPLock.lock()
        let tcpFlows = openTCPFlows
        openTCPLock.unlock()

        var closed = 0
        for (_, flow) in tcpFlows {
            guard claimTCPFlowClose(flow) else { continue }
            closed += 1
            unregisterLiveFlow(flow)
            flow.closeReadWithError(nil)
            flow.closeWriteWithError(nil)
        }

        udpLock.lock()
        let udpFlows = Array(udpAssociations.keys)
        udpLock.unlock()
        for flow in udpFlows {
            teardownUDP(flow)
        }

        // handshakes still in flight belong to the old config too
        sweepHandshakes(all: true)

        log("Closed \(closed) TCP and \(udpFlows.count) UDP flow(s): \(reason)", level: "INFO")
    }

    private func registerLiveFlow(_ flow: NEAppProxyFlow, pid: Int, process: String, destIp: String, host: String, port: UInt16, isUdp: Bool, proxyLabel: String) {
        let key = ObjectIdentifier(flow)
        activeFlowsLock.lock()
        let seq = nextFlowSeq
        nextFlowSeq += 1
        activeFlows[key] = LiveFlowInfo(id: "ne-\(seq)", pid: pid, processName: process, destIp: destIp, host: host, port: port, isUdp: isUdp, proxyLabel: proxyLabel, sent: 0, received: 0, startedAt: Date())
        activeFlowsLock.unlock()
    }

    private func addFlowBytes(_ flow: NEAppProxyFlow, sentDelta: Int64, recvDelta: Int64) {
        let key = ObjectIdentifier(flow)
        activeFlowsLock.lock()
        if var info = activeFlows[key] {
            info.sent += sentDelta
            info.received += recvDelta
            activeFlows[key] = info
        }
        activeFlowsLock.unlock()
    }

    private func updateUDPFlowEndpoint(_ flow: NEAppProxyFlow, destIp: String, host: String, port: UInt16) {
        let key = ObjectIdentifier(flow)
        activeFlowsLock.lock()
        if var info = activeFlows[key], info.port == 0 {
            info.destIp = destIp
            info.host = host
            info.port = port
            activeFlows[key] = info
        }
        activeFlowsLock.unlock()
    }

    private func unregisterLiveFlow(_ flow: NEAppProxyFlow) {
        let key = ObjectIdentifier(flow)
        activeFlowsLock.lock()
        activeFlows.removeValue(forKey: key)
        activeFlowsLock.unlock()
    }

    private func closeRelay(_ flow: NEAppProxyTCPFlow, _ connection: NWTCPConnection, error: Error? = nil) {
        _ = completeHandshake(connection)
        connection.cancel()
        unregisterLiveFlow(flow)
        // a config reset may have closed this flow already
        guard claimTCPFlowClose(flow) else { return }
        flow.closeReadWithError(error)
        flow.closeWriteWithError(error)
    }
    
    override func handleAppMessage(_ messageData: Data, completionHandler: ((Data?) -> Void)?) {
        guard let message = try? JSONSerialization.jsonObject(with: messageData) as? [String: Any],
              let action = message["action"] as? String else {
            completionHandler?(nil)
            return
        }
        
        switch action {
        case "getFlows":
            let now = Date()
            activeFlowsLock.lock()
            let list: [[String: Any]] = activeFlows.values.map { f in
                [
                    "id": f.id,
                    "pid": f.pid,
                    "processName": f.processName,
                    "destIp": f.destIp,
                    "host": f.host,
                    "port": Int(f.port),
                    "isUdp": f.isUdp,
                    "proxyLabel": f.proxyLabel,
                    "sent": f.sent,
                    "received": f.received,
                    "ageMs": Int(now.timeIntervalSince(f.startedAt) * 1000),
                ]
            }
            activeFlowsLock.unlock()
            completionHandler?(try? JSONSerialization.data(withJSONObject: list))
        case "getLogs":
            logQueueLock.lock()
            if logCount > 0 {
                let batchSize = min(100, logCount)
                var logsToSend: [[String: String]] = []
                logsToSend.reserveCapacity(batchSize)
                for _ in 0..<batchSize {
                    if let entry = logBuffer[logHead] {
                        logsToSend.append(entry.toDict())
                    }
                    logBuffer[logHead] = nil  // release the drained entry
                    logHead = (logHead + 1) % AppProxyProvider.logCapacity
                    logCount -= 1
                }
                logQueueLock.unlock()
                completionHandler?(try? JSONSerialization.data(withJSONObject: logsToSend))
            } else {
                logQueueLock.unlock()
                completionHandler?(nil)
            }
        case "setEnabled":
            if let enabled = message["enabled"] as? Bool {
                engineEnabled = enabled
                if !enabled {
                    // pausing has to release the connections that are already
                    // proxied, otherwise they keep flowing through the proxy
                    resetOpenFlows(reason: "engine paused")
                }
                completionHandler?(try? JSONSerialization.data(withJSONObject: ["status": "ok"]))
            } else {
                completionHandler?(nil)
            }
        case "setTrafficLogging":
            if let enabled = message["enabled"] as? Bool {
                trafficLoggingEnabled = enabled
                let response = ["status": "ok"]
                completionHandler?(try? JSONSerialization.data(withJSONObject: response))
            } else {
                completionHandler?(nil)
            }
        case "setProxyConfigs":
            if let configs = message["configs"] as? [[String: Any]] {
                proxyLock.lock()
                let previousConfigs = storedProxyConfigs
                let previousDefault = defaultProxyId
                storedProxyConfigs = [:]
                defaultProxyId = message["defaultProxyId"] as? String ?? ""
                warnedKeys.removeAll()
                for configDict in configs {
                    guard let id = configDict["id"] as? String,
                          let type = configDict["proxyType"] as? String,
                          let host = configDict["proxyHost"] as? String,
                          let port = configDict["proxyPort"] as? Int else { continue }
                    storedProxyConfigs[id] = StoredProxyConfig(
                        type: type, host: host, port: port,
                        username: configDict["proxyUsername"] as? String,
                        password: configDict["proxyPassword"] as? String,
                        // an app that doesn't send the key keeps the old behavior (ip)
                        sendDomain: configDict["sendDomainToProxy"] as? Bool ?? false
                    )
                }
                // a changed proxy list or default invalidates every flow that is
                // still running on the old one. the reset waits for setRules so
                // the reconnecting flows already match the new rule list.
                let changed = defaultProxyId != previousDefault || storedProxyConfigs != previousConfigs
                if changed { pendingFlowReset = true }
                proxyLock.unlock()
                log("Proxy configs updated: \(storedProxyConfigs.count) config(s)")
            }
            completionHandler?(try? JSONSerialization.data(withJSONObject: ["status": "ok"]))

        case "addRule":
            if let ruleData = try? JSONSerialization.data(withJSONObject: message),
               var rule = try? JSONDecoder().decode(ProxyRule.self, from: ruleData) {
                rulesLock.lock()
                rule.ruleId = nextRuleId
                nextRuleId += 1
                rules.append((rule, ProxyRule.Matcher(rule)))
                rulesLock.unlock()

                let response: [String: Any] = [
                    "status": "ok",
                    "ruleId": rule.ruleId,
                    "processNames": rule.processNames,
                    "targetHosts": rule.targetHosts,
                    "targetPorts": rule.targetPorts,
                    "protocol": rule.ruleProtocol.rawValue,
                    "action": rule.action,
                    "enabled": rule.enabled
                ]
                completionHandler?(try? JSONSerialization.data(withJSONObject: response))
            } else {
                let response = ["status": "error", "message": "Invalid rule format"]
                completionHandler?(try? JSONSerialization.data(withJSONObject: response))
            }
        
        case "setRules":
            // replace the whole rule list in one step. the old way was clearRules
            // followed by one addRule per rule, and any connection that arrived
            // in between was matched against a half-built list (so it went direct)
            guard let raw = message["rules"] as? [[String: Any]] else {
                completionHandler?(try? JSONSerialization.data(withJSONObject: ["status": "error", "message": "Invalid rules payload"]))
                return
            }
            var built: [CompiledRule] = []
            built.reserveCapacity(raw.count)
            for dict in raw {
                guard let ruleData = try? JSONSerialization.data(withJSONObject: dict),
                      let rule = try? JSONDecoder().decode(ProxyRule.self, from: ruleData) else { continue }
                built.append((rule, ProxyRule.Matcher(rule)))
            }
            rulesLock.lock()
            rules = built.map { item in
                var rule = item.rule
                rule.ruleId = nextRuleId
                nextRuleId += 1
                return (rule, item.matcher)
            }
            rulesLock.unlock()
            // both halves of the config are in place now, so flows still running
            // on the previous proxy can be dropped and will reconnect against
            // the new one instead of keeping the old exit ip
            let signature = (try? JSONSerialization.data(withJSONObject: raw, options: [.sortedKeys]))
                .map { String(decoding: $0, as: UTF8.self) } ?? ""
            let wantsLocalhost = built.contains { $0.rule.enabled && $0.rule.localhostViaProxy && $0.rule.action != "DIRECT" && $0.rule.action != "BLOCK" }
            proxyLock.lock()
            let shouldReset = pendingFlowReset
            pendingFlowReset = false
            let rulesChanged = signature != lastRulesSignature
            lastRulesSignature = signature
            configLoaded = true
            let localhostChanged = wantsLocalhost != localhostInterceptEnabled
            localhostInterceptEnabled = wantsLocalhost
            proxyLock.unlock()
            if localhostChanged {
                let updatedSettings = makeTunnelSettings(includeLocalhost: wantsLocalhost)
                self.setTunnelNetworkSettings(updatedSettings) { [weak self] error in
                    if let error = error {
                        self?.diag("Could not update localhost tunnel settings: \(error.localizedDescription)", level: "ERROR")
                    }
                }
            }
            if shouldReset || rulesChanged {
                resetOpenFlows(reason: shouldReset ? "proxy configuration changed" : "rules changed")
            }
            let response: [String: Any] = ["status": "ok", "count": built.count, "skipped": raw.count - built.count]
            completionHandler?(try? JSONSerialization.data(withJSONObject: response))

        case "clearRules":
            rulesLock.lock()
            let count = rules.count
            rules.removeAll()
            rulesLock.unlock()
            let response: [String: Any] = ["status": "ok", "cleared": count]
            completionHandler?(try? JSONSerialization.data(withJSONObject: response))
        
        case "clearConfig":
            // drop the stored proxy configs so credentials don't sit in memory after stop
            engineEnabled = false
            proxyLock.lock()
            storedProxyConfigs.removeAll()
            warnedKeys.removeAll()
            // leaving the old default id behind made the next setProxyConfigs
            // compare against a default that no longer exists
            defaultProxyId = ""
            configLoaded = false
            pendingFlowReset = false
            lastRulesSignature = ""
            localhostInterceptEnabled = false
            proxyLock.unlock()
            completionHandler?(try? JSONSerialization.data(withJSONObject: ["status": "ok"]))
        default:
            completionHandler?(nil)
        }
    }
    
    override func sleep(completionHandler: @escaping () -> Void) {
        completionHandler()
    }
    
    override func wake() {
    }
    
    private func isConfigLoaded() -> Bool {
        proxyLock.lock()
        defer { proxyLock.unlock() }
        return configLoaded
    }

    private func resolveProxyConfig(forAction action: String) -> StoredProxyConfig? {
        proxyLock.lock()
        defer { proxyLock.unlock() }
        let key = action == "PROXY" ? defaultProxyId : action
        return key.isEmpty ? nil : storedProxyConfigs[key]
    }

    override func handleNewFlow(_ flow: NEAppProxyFlow) -> Bool {
        guard engineEnabled else { return false }
        if let tcpFlow = flow as? NEAppProxyTCPFlow {
            return handleTCPFlow(tcpFlow)
        } else if let udpFlow = flow as? NEAppProxyUDPFlow {
            return handleUDPFlow(udpFlow)
        }
        return false
    }
    
    private func handleTCPFlow(_ flow: NEAppProxyTCPFlow) -> Bool {
        let metaData = flow.metaData
        let processPath = metaData.sourceAppSigningIdentifier
        
        // never proxy our own traffic, it would loop
        if processPath == "dev.anyproxy.app" || processPath == "dev.anyproxy.app.extension" {
            return false
        }

        let remoteEndpoint = flow.remoteEndpoint
        var destination = ""
        var portNum: UInt16 = 0
        var portStr = ""
        
        if let remoteHost = remoteEndpoint as? NWHostEndpoint {
            destination = remoteHost.hostname
            portStr = remoteHost.port
            portNum = UInt16(portStr) ?? 0
        } else {
            destination = String(describing: remoteEndpoint)
            portStr = "unknown"
        }

        let isLoopback = Self.isLoopbackAddress(destination)
        if Self.isNonRoutableNonLoopback(destination) || (isLoopback && portNum == 53) {
            return false
        }
        
        let identity = getProcessIdentity(from: metaData)
        let pid = identity?.pid ?? 0
        let processName = identity?.name
        let displayName = processName ?? processPath
        
        // domains that resolved to this ip (from the dns proxy), used for domain
        // rules and to show the hostname in the log instead of the raw ip
        let domains = DNSMapStore.shared.domains(forIP: destination)
        let logDest = domains.first ?? destination

        if !isConfigLoaded() {
            warnOnce("unconfigured", "Network Extension is waiting for proxy configuration; blocking traffic to prevent direct fallback")
            sendLogToApp(protocol: "TCP", pid: pid, process: displayName, destination: logDest, port: portStr, proxy: "BLOCK")
            flow.closeReadWithError(FlowFailure.refused)
            flow.closeWriteWithError(FlowFailure.refused)
            return true
        }

        // rules are always evaluated. a proxy config is only needed by a rule that
        // routes to one, BLOCK and DIRECT rules work without any (they used to be
        // skipped whenever no proxy was configured, so a lone BLOCK rule did nothing)
        let matchedRule = findMatchingRule(bundleId: processPath, processName: processName, executablePath: identity?.path, ancestorPaths: identity?.ancestorPaths ?? [], destination: destination, port: portNum, connectionProtocol: .tcp, checkIpPort: true, domains: domains)

        if let rule = matchedRule {
            switch rule.action {
            case "DIRECT":
                sendLogToApp(protocol: "TCP", pid: pid, process: displayName, destination: logDest, port: portStr, proxy: "Direct")
                return false
            case "BLOCK":
                sendLogToApp(protocol: "TCP", pid: pid, process: displayName, destination: logDest, port: portStr, proxy: "BLOCK")
                flow.closeReadWithError(FlowFailure.refused)
                flow.closeWriteWithError(FlowFailure.refused)
                return true
            default:
                if isLoopback && !rule.localhostViaProxy {
                    sendLogToApp(protocol: "TCP", pid: pid, process: displayName, destination: logDest, port: portStr, proxy: "Direct")
                    return false
                }
                guard let config = resolveProxyConfig(forAction: rule.action) else {
                    warnOnce("missing:\(rule.action)", "Rule references an unavailable proxy; blocking TCP flow to prevent direct fallback")
                    sendLogToApp(protocol: "TCP", pid: pid, process: displayName, destination: logDest, port: portStr, proxy: "BLOCK")
                    flow.closeReadWithError(FlowFailure.refused)
                    flow.closeWriteWithError(FlowFailure.refused)
                    return true
                }
                // never loop traffic addressed to the local proxy server itself
                if isLoopback && Self.isLoopbackAddress(config.host) && portNum == UInt16(config.port) {
                    return false
                }
                sendLogToApp(protocol: "TCP", pid: pid, process: displayName, destination: logDest, port: portStr, proxy: proxyLabel(config))
                registerLiveFlow(flow, pid: pid, process: displayName, destIp: destination, host: logDest, port: portNum, isUdp: false, proxyLabel: proxyLabel(config))
                trackTCPFlow(flow)
                proxyTCPFlow(flow, destination: destination, port: portNum, config: config)
                return true
            }
        } else {
            if isLoopback {
                return false
            }
            proxyLock.lock()
            let defId = defaultProxyId
            let defConfig = defId.isEmpty ? nil : storedProxyConfigs[defId]
            proxyLock.unlock()
            if let config = defConfig {
                sendLogToApp(protocol: "TCP", pid: pid, process: displayName, destination: logDest, port: portStr, proxy: proxyLabel(config))
                registerLiveFlow(flow, pid: pid, process: displayName, destIp: destination, host: logDest, port: portNum, isUdp: false, proxyLabel: proxyLabel(config))
                trackTCPFlow(flow)
                proxyTCPFlow(flow, destination: destination, port: portNum, config: config)
                return true
            }
            if !defId.isEmpty {
                warnOnce("missing-default:\(defId)", "Active default proxy is unavailable; blocking TCP flow to prevent direct fallback")
                sendLogToApp(protocol: "TCP", pid: pid, process: displayName, destination: logDest, port: portStr, proxy: "BLOCK")
                flow.closeReadWithError(FlowFailure.refused)
                flow.closeWriteWithError(FlowFailure.refused)
                return true
            }
            sendLogToApp(protocol: "TCP", pid: pid, process: displayName, destination: logDest, port: portStr, proxy: "Direct")
            return false
        }
    }

    // human readable label for the connection log, e.g. SOCKS5 127.0.0.1:1080
    private func proxyLabel(_ config: StoredProxyConfig) -> String {
        return "\(config.type.uppercased()) \(config.host):\(config.port)"
    }
    
    private func handleUDPFlow(_ flow: NEAppProxyUDPFlow) -> Bool {
        let metaData = flow.metaData
        let processPath = metaData.sourceAppSigningIdentifier
        let identity = getProcessIdentity(from: metaData)
        let pid = identity?.pid ?? 0
        let processName = identity?.name
        let displayName = processName ?? processPath
        
        if processPath == "dev.anyproxy.app" || processPath == "dev.anyproxy.app.extension" {
            return false
        }

        if !isConfigLoaded() {
            warnOnce("unconfigured", "Network Extension is waiting for proxy configuration; blocking traffic to prevent direct fallback")
            sendLogToApp(protocol: "UDP", pid: pid, process: displayName, destination: "unknown", port: "unknown", proxy: "BLOCK")
            flow.closeReadWithError(FlowFailure.refused)
            flow.closeWriteWithError(FlowFailure.refused)
            return true
        }
        
        // rules first: udp BLOCK and DIRECT need no proxy at all. only a rule that
        // sends udp through a proxy needs a socks5 one (http proxies can't carry udp)
        proxyLock.lock()
        let hasAnySocks5 = storedProxyConfigs.values.contains { $0.type.lowercased() == "socks5" }
        proxyLock.unlock()

        let matchedRule = findMatchingRule(bundleId: processPath, processName: processName, executablePath: identity?.path, ancestorPaths: identity?.ancestorPaths ?? [], destination: "", port: 0, connectionProtocol: .udp, checkIpPort: false)

        if let rule = matchedRule {
            let action = rule.action
            switch action {
            case "DIRECT":
                sendLogToApp(protocol: "UDP", pid: pid, process: displayName, destination: "unknown", port: "unknown", proxy: "Direct")
                return false
            case "BLOCK":
                sendLogToApp(protocol: "UDP", pid: pid, process: displayName, destination: "unknown", port: "unknown", proxy: "BLOCK")
                // a handled flow that is never closed stays pinned in the extension
                flow.closeReadWithError(FlowFailure.refused)
                flow.closeWriteWithError(FlowFailure.refused)
                return true
            default:
                guard let socks5Config = resolveProxyConfig(forAction: action) else {
                    warnOnce("missing:\(action)", "Rule references an unavailable proxy; blocking UDP flow to prevent direct fallback")
                    sendLogToApp(protocol: "UDP", pid: pid, process: displayName, destination: "unknown", port: "unknown", proxy: "BLOCK")
                    flow.closeReadWithError(FlowFailure.refused)
                    flow.closeWriteWithError(FlowFailure.refused)
                    return true
                }
                guard socks5Config.type.lowercased() == "socks5" else {
                    warnOnce("udp-http:\(action)", "HTTP/HTTPS proxy does not support UDP; blocking UDP flow to prevent direct fallback")
                    sendLogToApp(protocol: "UDP", pid: pid, process: displayName, destination: "unknown", port: "unknown", proxy: "BLOCK")
                    flow.closeReadWithError(FlowFailure.refused)
                    flow.closeWriteWithError(FlowFailure.refused)
                    return true
                }
                let label = proxyLabel(socks5Config)
                flow.open(withLocalEndpoint: nil) { [weak self] error in
                    guard let self = self else { return }
                    // a closed flow here is normal churn (quic opening and dropping
                    // udp/443 flows constantly), nothing to do and not worth logging
                    if error != nil {
                        return
                    }
                    self.registerLiveFlow(flow, pid: pid, process: displayName, destIp: "0.0.0.0", host: "UDP association", port: 0, isUdp: true, proxyLabel: label)
                    // pass the resolved display name (curl), not the raw signing id (curl-<hash>)
                    self.proxyUDPFlowViaSOCKS5(flow, displayName: displayName, socksHost: socks5Config.host, socksPort: socks5Config.port, username: socks5Config.username, password: socks5Config.password)
                }
                return true
            }
        } else {
            proxyLock.lock()
            let defId = defaultProxyId
            let defConfig = defId.isEmpty ? nil : storedProxyConfigs[defId]
            proxyLock.unlock()
            if let socks5Config = defConfig, socks5Config.type.lowercased() == "socks5" {
                let label = proxyLabel(socks5Config)
                flow.open(withLocalEndpoint: nil) { [weak self] error in
                    guard let self = self, error == nil else { return }
                    self.registerLiveFlow(flow, pid: pid, process: displayName, destIp: "0.0.0.0", host: "UDP association", port: 0, isUdp: true, proxyLabel: label)
                    self.proxyUDPFlowViaSOCKS5(flow, displayName: displayName, socksHost: socks5Config.host, socksPort: socks5Config.port, username: socks5Config.username, password: socks5Config.password)
                }
                return true
            }
            if !defId.isEmpty {
                if defConfig == nil {
                    warnOnce("missing-default:\(defId)", "Active default proxy is unavailable; blocking UDP flow to prevent direct fallback")
                } else {
                    warnOnce("udp-http:default", "HTTP/HTTPS proxy does not support UDP; blocking UDP flow to prevent direct fallback")
                }
                sendLogToApp(protocol: "UDP", pid: pid, process: displayName, destination: "unknown", port: "unknown", proxy: "BLOCK")
                flow.closeReadWithError(FlowFailure.refused)
                flow.closeWriteWithError(FlowFailure.refused)
                return true
            }
            // no rule matched. as before, only log it when udp proxying is possible
            // at all, otherwise every mDNS/NTP flow would fill the log
            if hasAnySocks5 {
                sendLogToApp(protocol: "UDP", pid: pid, process: displayName, destination: "unknown", port: "unknown", proxy: "Direct")
            }
            return false
        }
    }
    
    // handshake failed before an association existed, so drop the control
    // connection and close the flow instead of leaking them
    private func failUDPHandshake(_ clientFlow: NEAppProxyUDPFlow, _ tcpConnection: NWTCPConnection, _ reason: String) {
        unregisterLiveFlow(clientFlow)
        log(reason, level: "ERROR")
        tcpConnection.cancel()
        clientFlow.closeReadWithError(nil)
        clientFlow.closeWriteWithError(nil)
    }

    private func proxyUDPFlowViaSOCKS5(_ clientFlow: NEAppProxyUDPFlow, displayName: String, socksHost: String, socksPort: Int, username: String?, password: String?) {
        let proxyEndpoint = NWHostEndpoint(hostname: socksHost, port: String(socksPort))
        let tcpConnection = createTCPConnection(to: proxyEndpoint, enableTLS: false, tlsParameters: nil, delegate: nil)
        armHandshakeTimeout(flow: clientFlow, connection: tcpConnection)

        // offer username/password auth when we have credentials, same as the tcp path
        let useAuth = (username != nil && password != nil)
        let greeting: [UInt8] = useAuth ? [0x05, 0x02, 0x00, 0x02] : [0x05, 0x01, 0x00]

        tcpConnection.write(Data(greeting)) { [weak self] error in
            guard let self = self else { return }

            if let error = error {
                self.failUDPHandshake(clientFlow, tcpConnection, "SOCKS5 UDP greeting failed: \(error.localizedDescription)")
                return
            }

            tcpConnection.readMinimumLength(2, maximumLength: 2) { [weak self] data, error in
                guard let self = self else { return }

                if let error = error {
                    self.failUDPHandshake(clientFlow, tcpConnection, "SOCKS5 UDP greeting response failed: \(error.localizedDescription)")
                    return
                }

                guard let data = data, data.count == 2, data[0] == 0x05 else {
                    self.failUDPHandshake(clientFlow, tcpConnection, "SOCKS5 UDP greeting response invalid")
                    return
                }

                switch data[1] {
                case 0x00:
                    self.sendSOCKS5UDPAssociate(clientFlow: clientFlow, tcpConnection: tcpConnection, socksHost: socksHost, displayName: displayName)
                case 0x02:
                    self.sendSOCKS5UDPAuth(clientFlow: clientFlow, tcpConnection: tcpConnection, socksHost: socksHost, displayName: displayName, username: username ?? "", password: password ?? "")
                default:
                    self.failUDPHandshake(clientFlow, tcpConnection, "SOCKS5 UDP no acceptable auth method")
                }
            }
        }
    }

    private func sendSOCKS5UDPAuth(clientFlow: NEAppProxyUDPFlow, tcpConnection: NWTCPConnection, socksHost: String, displayName: String, username: String, password: String) {
        let user = Array(username.utf8), pass = Array(password.utf8)
        guard user.count <= 255, pass.count <= 255 else {
            self.failUDPHandshake(clientFlow, tcpConnection, "SOCKS5 UDP credentials too long")
            return
        }

        var authData = Data()
        authData.append(0x01)
        authData.append(UInt8(user.count))
        authData.append(contentsOf: user)
        authData.append(UInt8(pass.count))
        authData.append(contentsOf: pass)

        tcpConnection.write(authData) { [weak self] error in
            guard let self = self else { return }

            if let error = error {
                self.failUDPHandshake(clientFlow, tcpConnection, "SOCKS5 UDP auth write failed: \(error.localizedDescription)")
                return
            }

            tcpConnection.readMinimumLength(2, maximumLength: 2) { [weak self] data, error in
                guard let self = self else { return }

                if let error = error {
                    self.failUDPHandshake(clientFlow, tcpConnection, "SOCKS5 UDP auth response failed: \(error.localizedDescription)")
                    return
                }

                guard let data = data, data.count == 2, data[1] == 0x00 else {
                    self.failUDPHandshake(clientFlow, tcpConnection, "SOCKS5 UDP auth rejected")
                    return
                }

                self.sendSOCKS5UDPAssociate(clientFlow: clientFlow, tcpConnection: tcpConnection, socksHost: socksHost, displayName: displayName)
            }
        }
    }

    private func sendSOCKS5UDPAssociate(clientFlow: NEAppProxyUDPFlow, tcpConnection: NWTCPConnection, socksHost: String, displayName: String) {
        var request = Data()
        request.append(0x05)
        request.append(0x03)
        request.append(0x00)
        request.append(0x01)
        request.append(contentsOf: [0, 0, 0, 0])
        request.append(contentsOf: [0, 0])
        
        tcpConnection.write(request) { [weak self] error in
            guard let self = self else { return }

            if let error = error {
                self.failUDPHandshake(clientFlow, tcpConnection, "SOCKS5 UDP ASSOCIATE failed: \(error.localizedDescription)")
                return
            }

            self.readSOCKS5Reply(tcpConnection) { [weak self] reply in
                guard let self = self else { return }

                switch reply {
                case .failed(let reason):
                    self.failUDPHandshake(clientFlow, tcpConnection, "SOCKS5 UDP ASSOCIATE failed: \(reason)")
                case .rejected(let rep):
                    self.failUDPHandshake(clientFlow, tcpConnection, "SOCKS5 UDP ASSOCIATE rejected, REP=0x\(String(format: "%02x", rep)) (\(self.socksReplyReason(rep)))")
                case .success(let data):
                    let (parsedHost, relayPort) = self.parseSOCKS5Address(from: data, offset: 3)
                    // many socks servers reply 0.0.0.0 meaning "reuse the control connection host"
                    let relayHost = (parsedHost == "0.0.0.0" || parsedHost == "::" || parsedHost.isEmpty) ? socksHost : parsedHost
                    guard relayPort != 0 else {
                        self.failUDPHandshake(clientFlow, tcpConnection, "SOCKS5 UDP ASSOCIATE returned no relay port")
                        return
                    }
                    self.relayUDPThroughSOCKS5(clientFlow: clientFlow, relayHost: relayHost, relayPort: relayPort, tcpConnection: tcpConnection, displayName: displayName)
                }
            }
        }
    }

    private enum SOCKS5Reply {
        case success(Data)       // the whole reply, VER REP RSV ATYP BND.ADDR BND.PORT
        case rejected(UInt8)     // REP != 0
        case failed(String)
    }

    // reads exactly n bytes, never more, so nothing the server sends after what we
    // asked for (a banner from a server-first protocol) is consumed here
    private func readExact(_ connection: NWTCPConnection, _ n: Int, completion: @escaping (Data?, Error?) -> Void) {
        connection.readMinimumLength(n, maximumLength: n) { data, error in
            if let error = error { completion(nil, error); return }
            guard let data = data, data.count == n else { completion(nil, nil); return }
            completion(data, nil)
        }
    }

    // reads one SOCKS5 reply. the length depends on the address type (10 bytes for
    // ipv4, 22 for ipv6, variable for a domain), reading a fixed 10 used to leave
    // part of an ipv6/domain reply in the client's data and could swallow the
    // first bytes the real server sent
    private func readSOCKS5Reply(_ connection: NWTCPConnection, completion: @escaping (SOCKS5Reply) -> Void) {
        readExact(connection, 4) { [weak self] header, error in
            guard let self = self else { return }
            if let error = error { completion(.failed(error.localizedDescription)); return }
            guard let header = header, header[0] == 0x05 else { completion(.failed("invalid reply")); return }
            guard header[1] == 0x00 else { completion(.rejected(header[1])); return }

            func finish(_ rest: Data?, _ error: Error?, prefix: Data) {
                if let error = error { completion(.failed(error.localizedDescription)); return }
                guard let rest = rest else { completion(.failed("short reply")); return }
                completion(.success(prefix + rest))
            }

            switch header[3] {
            case 0x01:
                self.readExact(connection, 6) { rest, error in finish(rest, error, prefix: header) }
            case 0x04:
                self.readExact(connection, 18) { rest, error in finish(rest, error, prefix: header) }
            case 0x03:
                self.readExact(connection, 1) { lenByte, error in
                    guard let lenByte = lenByte else { completion(.failed(error?.localizedDescription ?? "short reply")); return }
                    self.readExact(connection, Int(lenByte[0]) + 2) { rest, error in
                        finish(rest, error, prefix: header + lenByte)
                    }
                }
            default:
                completion(.failed("unsupported address type 0x\(String(format: "%02x", header[3]))"))
            }
        }
    }

    // reads an http proxy's reply up to the end of its headers. returns the status
    // code and whatever the server already sent after the headers, which belongs to
    // the tunnel and has to reach the client first
    private func readHTTPConnectResponse(_ connection: NWTCPConnection, buffer: Data = Data(), completion: @escaping (Int?, Data, String) -> Void) {
        connection.readMinimumLength(1, maximumLength: 8192) { [weak self] data, error in
            guard let self = self else { return }
            if let error = error { completion(nil, Data(), error.localizedDescription); return }
            var buffer = buffer
            if let data = data { buffer.append(data) }

            if let end = buffer.range(of: Data([0x0d, 0x0a, 0x0d, 0x0a])) {
                let head = String(decoding: buffer[buffer.startIndex..<end.lowerBound], as: UTF8.self)
                let statusLine = head.components(separatedBy: "\r\n").first ?? ""
                let parts = statusLine.split(separator: " ", omittingEmptySubsequences: true)
                guard parts.count >= 2, parts[0].hasPrefix("HTTP/1."), let status = Int(parts[1]) else {
                    completion(nil, Data(), "invalid status line: \(statusLine.prefix(80))")
                    return
                }
                completion(status, Data(buffer[end.upperBound...]), statusLine)
            } else if buffer.count >= 16384 || data == nil || data!.isEmpty {
                completion(nil, Data(), "response headers incomplete or too large")
            } else {
                self.readHTTPConnectResponse(connection, buffer: buffer, completion: completion)
            }
        }
    }

    // human readable name for a socks5 REP code, from RFC 1928
    private func socksReplyReason(_ rep: UInt8) -> String {
        switch rep {
        case 0x01: return "general failure"
        case 0x02: return "not allowed by ruleset"
        case 0x03: return "network unreachable"
        case 0x04: return "host unreachable"
        case 0x05: return "connection refused"
        case 0x06: return "TTL expired"
        case 0x07: return "command not supported"
        case 0x08: return "address type not supported"
        default: return "unknown"
        }
    }

    private func parseSOCKS5Address(from data: Data, offset: Int) -> (String, UInt16) {
        guard data.count > offset else { return ("0.0.0.0", 0) }
        let atyp = data[offset]

        if atyp == 0x01 {
            guard data.count >= offset + 7 else { return ("0.0.0.0", 0) }
            let ip = "\(data[offset+1]).\(data[offset+2]).\(data[offset+3]).\(data[offset+4])"
            let port = (UInt16(data[offset+5]) << 8) | UInt16(data[offset+6])
            return (ip, port)
        } else if atyp == 0x04 {
            guard data.count >= offset + 19 else { return ("0.0.0.0", 0) }
            var ipv6Parts: [String] = []
            for i in 0..<8 {
                let idx = offset + 1 + (i * 2)
                let part = (UInt16(data[idx]) << 8) | UInt16(data[idx+1])
                ipv6Parts.append(String(format: "%x", part))
            }
            let ip = ipv6Parts.joined(separator: ":")
            let port = (UInt16(data[offset+17]) << 8) | UInt16(data[offset+18])
            return (ip, port)
        } else if atyp == 0x03 {
            guard data.count >= offset + 2 else { return ("0.0.0.0", 0) }
            let len = Int(data[offset+1])
            guard data.count >= offset + 2 + len + 2 else { return ("0.0.0.0", 0) }
            let domain = String(data: data[(offset+2)..<(offset+2+len)], encoding: .utf8) ?? "unknown"
            let port = (UInt16(data[offset+2+len]) << 8) | UInt16(data[offset+2+len+1])
            return (domain, port)
        }

        return ("0.0.0.0", 0)
    }
    
    private func relayUDPThroughSOCKS5(clientFlow: NEAppProxyUDPFlow, relayHost: String, relayPort: UInt16, tcpConnection: NWTCPConnection, displayName: String) {
        guard completeHandshake(tcpConnection) else { return }
        let relayEndpoint = NWHostEndpoint(hostname: relayHost, port: String(relayPort))
        let udpSession = self.createUDPSession(to: relayEndpoint, from: nil)

        let association = UDPAssociation(clientFlow: clientFlow, controlConnection: tcpConnection, udpSession: udpSession, displayName: displayName)

        udpLock.lock()
        let existing = udpAssociations[clientFlow]
        udpAssociations[clientFlow] = association
        udpLock.unlock()
        // if the app reused a flow, drop the stale association first
        existing?.release()

        readAndForwardClientUDP(association)
        readAndForwardRelayUDP(association)
        // note: we deliberately don't read the control connection to detect close.
        // holding a strong ref keeps the associate alive, and reading it risks a
        // spurious teardown that would EPIPE the app's udp socket.
    }

    // idempotent, cancels both channels and closes the flow exactly once
    private func teardownUDP(_ flow: NEAppProxyUDPFlow) {
        unregisterLiveFlow(flow)
        udpLock.lock()
        guard let assoc = udpAssociations.removeValue(forKey: flow), !assoc.isTornDown else {
            udpLock.unlock()
            return
        }
        assoc.isTornDown = true
        udpLock.unlock()

        assoc.release()
        flow.closeReadWithError(nil)
        flow.closeWriteWithError(nil)
    }

    private func readAndForwardClientUDP(_ association: UDPAssociation) {
        let clientFlow = association.clientFlow
        clientFlow.readDatagrams { [weak self] datagrams, endpoints, error in
            guard let self = self else { return }

            if let error = error {
                self.log("UDP read error: \(error.localizedDescription)", level: "ERROR")
                self.teardownUDP(clientFlow)
                return
            }

            // nil or empty datagrams with no error = flow closed for reading.
            // re-arming on empty spins at 100% cpu because the read completes
            // immediately forever once the flow is closed (issue #89 / pr #90)
            guard let datagrams = datagrams, let endpoints = endpoints, !datagrams.isEmpty else {
                self.teardownUDP(clientFlow)
                return
            }

            association.touch()
            var toSend: [Data] = []
            toSend.reserveCapacity(datagrams.count)
            var sentPayloadBytes: Int64 = 0

            for i in 0..<min(datagrams.count, endpoints.count) {
                guard let nwHost = endpoints[i] as? NWHostEndpoint else { continue }
                let destHost = nwHost.hostname
                let destPort = UInt16(nwHost.port) ?? 0
                guard destPort != 0 else { continue }
                if Self.isLocalOrMulticastAddress(destHost) || destPort == 67 || destPort == 68 || destPort == 546 || destPort == 547 {
                    continue
                }

                // log each distinct destination once, cap the set so it can't grow forever
                if !association.loggedDestinations.contains(destHost) {
                    if association.loggedDestinations.count < 64 {
                        association.loggedDestinations.insert(destHost)
                    }
                    let logHost = DNSMapStore.shared.domains(forIP: destHost).first ?? destHost
                    self.updateUDPFlowEndpoint(clientFlow, destIp: destHost, host: logHost, port: destPort)
                    self.sendLogToApp(protocol: "UDP", process: association.displayName, destination: logHost, port: nwHost.port, proxy: "SOCKS5")
                }

                let key = UDPAssociation.HostPort(host: destHost, port: destPort)
                var header = association.headerCache[key]
                if header == nil, let built = self.socks5UDPHeader(destHost: destHost, destPort: destPort) {
                    if association.headerCache.count < UDPAssociation.cacheLimit { association.headerCache[key] = built }
                    header = built
                }
                // a udp payload can be up to ~65507 bytes, don't drop mtu-sized quic packets
                if let header = header, datagrams[i].count <= 65507 {
                    var packet = Data(capacity: header.count + datagrams[i].count)
                    packet.append(header)
                    packet.append(datagrams[i])
                    toSend.append(packet)
                    sentPayloadBytes += Int64(datagrams[i].count)
                }
            }

            if sentPayloadBytes > 0 {
                self.addFlowBytes(clientFlow, sentDelta: sentPayloadBytes, recvDelta: 0)
            }

            if !toSend.isEmpty {
                association.udpSession.writeMultipleDatagrams(toSend) { [weak self] error in
                    if let error = error {
                        self?.log("UDP write error: \(error.localizedDescription)", level: "ERROR")
                    }
                }
            }

            self.readAndForwardClientUDP(association)
        }
    }

    private func readAndForwardRelayUDP(_ association: UDPAssociation) {
        let clientFlow = association.clientFlow
        association.udpSession.setReadHandler({ [weak self, weak association] datagrams, error in
            guard let self = self, let association = association else { return }

            if let error = error {
                // stop reading the relay but don't close the app's flow, let it
                // time out on its own instead of getting a hard EPIPE
                self.log("UDP relay error: \(error.localizedDescription)", level: "ERROR")
                return
            }

            guard let datagrams = datagrams, !datagrams.isEmpty else { return }
            association.touch()

            var payloads: [Data] = []
            var endpoints: [NWEndpoint] = []
            payloads.reserveCapacity(datagrams.count)
            endpoints.reserveCapacity(datagrams.count)
            var recvPayloadBytes: Int64 = 0

            for datagram in datagrams {
                if let (payload, destHost, destPort) = self.decapsulateSOCKS5UDPWithEndpoint(datagram: datagram, association: association) {
                    payloads.append(payload)
                    endpoints.append(NWHostEndpoint(hostname: destHost, port: String(destPort)))
                    recvPayloadBytes += Int64(payload.count)
                }
            }

            if recvPayloadBytes > 0 {
                self.addFlowBytes(clientFlow, sentDelta: 0, recvDelta: recvPayloadBytes)
            }

            if !payloads.isEmpty {
                clientFlow.writeDatagrams(payloads, sentBy: endpoints) { [weak self] error in
                    if let error = error {
                        self?.log("UDP response write error: \(error.localizedDescription)", level: "ERROR")
                    }
                }
            }
        }, maxDatagrams: 32)
    }
    
    // the socks5 udp request header for one destination, the payload follows it
    private func socks5UDPHeader(destHost: String, destPort: UInt16) -> Data? {
        var header = Data()
        header.append(contentsOf: [0, 0])
        header.append(0x00)
        
        if let ipv4 = IPv4Address(destHost) {
            header.append(0x01)
            header.append(contentsOf: ipv4.rawValue)
        } else if let ipv6 = IPv6Address(destHost) {
            header.append(0x04)
            header.append(contentsOf: ipv6.rawValue)
        } else {
            // length prefix must be the utf8 byte count, not the character count
            let hostBytes = Array(destHost.utf8)
            guard hostBytes.count <= 255 else {
                self.log("Domain name too long: \(hostBytes.count) bytes", level: "ERROR")
                return nil
            }
            header.append(0x03)
            header.append(UInt8(hostBytes.count))
            header.append(contentsOf: hostBytes)
        }
        
        header.append(UInt8(destPort >> 8))
        header.append(UInt8(destPort & 0xFF))
        return header
    }
    
    private func decapsulateSOCKS5UDPWithEndpoint(datagram: Data, association: UDPAssociation) -> (Data, String, UInt16)? {
        guard datagram.count > 10 else { return nil }

        // byte 2 is FRAG, we don't reassemble so drop any fragmented datagram
        guard datagram[2] == 0x00 else { return nil }

        let atyp = datagram[3]
        var headerLen = 4
        var destHost = ""
        var destPort: UInt16 = 0

        if atyp == 0x01 {
            // ipv4 is 4 bytes + 2 port, already covered by count > 10. the dotted
            // string is cached by the raw 6 bytes, a flow talks to few peers
            let raw = (UInt64(datagram[4]) << 40) | (UInt64(datagram[5]) << 32) | (UInt64(datagram[6]) << 24)
                | (UInt64(datagram[7]) << 16) | (UInt64(datagram[8]) << 8) | UInt64(datagram[9])
            if let hit = association.endpointCache[raw] {
                destHost = hit.host
                destPort = hit.port
            } else {
                destHost = "\(datagram[4]).\(datagram[5]).\(datagram[6]).\(datagram[7])"
                destPort = (UInt16(datagram[8]) << 8) | UInt16(datagram[9])
                if association.endpointCache.count < UDPAssociation.cacheLimit {
                    association.endpointCache[raw] = (destHost, destPort)
                }
            }
            headerLen += 6
        } else if atyp == 0x04 {
            // ipv6 needs 16 bytes + 2 port
            guard datagram.count >= 22 else { return nil }
            var ipv6Parts: [String] = []
            for i in 0..<8 {
                let idx = 4 + (i * 2)
                let part = (UInt16(datagram[idx]) << 8) | UInt16(datagram[idx+1])
                ipv6Parts.append(String(format: "%x", part))
            }
            destHost = ipv6Parts.joined(separator: ":")
            destPort = (UInt16(datagram[20]) << 8) | UInt16(datagram[21])
            headerLen += 18
        } else if atyp == 0x03 {
            // byte 4 is the domain length, then the domain, then 2 port bytes
            guard datagram.count >= 6 else { return nil }
            let domainLen = Int(datagram[4])
            guard datagram.count >= 5 + domainLen + 2 else { return nil }
            destHost = String(data: datagram[5..<(5+domainLen)], encoding: .utf8) ?? "unknown"
            destPort = (UInt16(datagram[5+domainLen]) << 8) | UInt16(datagram[5+domainLen+1])
            headerLen += 1 + domainLen + 2
        } else {
            return nil
        }

        guard datagram.count > headerLen else { return nil }

        let payload = datagram[headerLen...]
        return (Data(payload), destHost, destPort)
    }
    
    // how long a DNS answer stays good enough to name a destination for the proxy
    private static let dnsNameMaxAge: TimeInterval = 300

    private func proxyTCPFlow(_ flow: NEAppProxyTCPFlow, destination originalDestination: String, port: UInt16, config: StoredProxyConfig) {
        // "let proxy resolve DNS", per proxy like the windows app: when the app
        // resolved this address through our DNS proxy, ask the proxy for the name
        // (socks5 domain address / CONNECT host:port) and fall back to the ip
        var destination = originalDestination
        if config.sendDomain,
           IPv4Address(originalDestination) != nil || IPv6Address(originalDestination) != nil,
           let name = DNSMapStore.shared.latestDomain(forIP: originalDestination, maxAge: AppProxyProvider.dnsNameMaxAge) {
            destination = name
        }
        let proxyEndpoint = NWHostEndpoint(hostname: config.host, port: String(config.port))
        let proxyConnection = createTCPConnection(to: proxyEndpoint, enableTLS: false, tlsParameters: nil, delegate: nil)
        armHandshakeTimeout(flow: flow, connection: proxyConnection)

        switch config.type.lowercased() {
        case "socks5":
            handleSOCKS5Proxy(clientFlow: flow, proxyConnection: proxyConnection, destination: destination, port: port, username: config.username, password: config.password)
        case "http":
            handleHTTPProxy(clientFlow: flow, proxyConnection: proxyConnection, destination: destination, port: port, username: config.username, password: config.password)
        default:
            log("unsupported proxy type: \(config.type)", level: "ERROR")
            closeRelay(flow, proxyConnection, error: FlowFailure.refused)
        }
    }
    
    private func handleSOCKS5Proxy(clientFlow: NEAppProxyTCPFlow, proxyConnection: NWTCPConnection, destination: String, port: UInt16, username: String?, password: String?) {
        var greeting: [UInt8]
        if username != nil && password != nil {
            greeting = [0x05, 0x02, 0x00, 0x02]
        } else {
            greeting = [0x05, 0x01, 0x00]
        }
        
        let greetingData = Data(greeting)
        proxyConnection.write(greetingData) { [weak self] error in
            if let error = error {
                self?.log("SOCKS5 greeting write failed: \(error.localizedDescription)", level: "ERROR")
                self?.closeRelay(clientFlow, proxyConnection, error: FlowFailure.proxyUnreachable)
                return
            }
            
            proxyConnection.readMinimumLength(2, maximumLength: 2) { [weak self] data, error in
                guard let self = self else { return }
                
                if let error = error {
                    self.log("SOCKS5 greeting response failed: \(error.localizedDescription)", level: "ERROR")
                    self.closeRelay(clientFlow, proxyConnection, error: FlowFailure.proxyUnreachable)
                    return
                }
                
                guard let data = data, data.count == 2 else {
                    self.log("SOCKS5 invalid greeting response", level: "ERROR")
                    self.closeRelay(clientFlow, proxyConnection, error: FlowFailure.proxyUnreachable)
                    return
                }
                
                let version = data[0]
                let method = data[1]
                
                if version != 0x05 {
                    self.log("SOCKS5 invalid version: \(version)", level: "ERROR")
                    self.closeRelay(clientFlow, proxyConnection, error: FlowFailure.proxyUnreachable)
                    return
                }
                
                if method == 0x00 {
                    self.sendSOCKS5ConnectRequest(clientFlow: clientFlow, proxyConnection: proxyConnection, destination: destination, port: port)
                } else if method == 0x02 {
                    self.sendSOCKS5Auth(clientFlow: clientFlow, proxyConnection: proxyConnection, destination: destination, port: port, username: username ?? "", password: password ?? "")
                } else {
                    self.log("SOCKS5 no acceptable auth method: \(method)", level: "ERROR")
                    self.closeRelay(clientFlow, proxyConnection, error: FlowFailure.refused)
                }
            }
        }
    }
    
    private func sendSOCKS5Auth(clientFlow: NEAppProxyTCPFlow, proxyConnection: NWTCPConnection, destination: String, port: UInt16, username: String, password: String) {
        let user = Array(username.utf8), pass = Array(password.utf8)
        guard user.count <= 255, pass.count <= 255 else {
            log("SOCKS5 credentials too long", level: "ERROR")
            closeRelay(clientFlow, proxyConnection, error: FlowFailure.refused)
            return
        }
        var authData = Data()
        authData.append(0x01)
        authData.append(UInt8(user.count))
        authData.append(contentsOf: user)
        authData.append(UInt8(pass.count))
        authData.append(contentsOf: pass)
        
        proxyConnection.write(authData) { [weak self] error in
            if let error = error {
                self?.log("SOCKS5 auth write failed: \(error.localizedDescription)", level: "ERROR")
                self?.closeRelay(clientFlow, proxyConnection, error: FlowFailure.proxyUnreachable)
                return
            }
            
            proxyConnection.readMinimumLength(2, maximumLength: 2) { [weak self] data, error in
                guard let self = self else { return }
                
                if let error = error {
                    self.log("SOCKS5 auth response failed: \(error.localizedDescription)", level: "ERROR")
                    self.closeRelay(clientFlow, proxyConnection, error: FlowFailure.proxyUnreachable)
                    return
                }
                
                guard let data = data, data.count == 2, data[1] == 0x00 else {
                    self.log("SOCKS5 auth failed", level: "ERROR")
                    self.closeRelay(clientFlow, proxyConnection, error: FlowFailure.refused)
                    return
                }
                
                self.sendSOCKS5ConnectRequest(clientFlow: clientFlow, proxyConnection: proxyConnection, destination: destination, port: port)
            }
        }
    }
    
    private func sendSOCKS5ConnectRequest(clientFlow: NEAppProxyTCPFlow, proxyConnection: NWTCPConnection, destination: String, port: UInt16) {
        var request = Data()
        request.append(0x05)
        request.append(0x01)
        request.append(0x00)
        
        if let ipAddr = IPv4Address(destination) {
            request.append(0x01)
            request.append(contentsOf: ipAddr.rawValue)
        } else if let ipAddr = IPv6Address(destination) {
            request.append(0x04)
            request.append(contentsOf: ipAddr.rawValue)
        } else {
            let hostBytes = Array(destination.utf8)
            guard hostBytes.count <= 255 else {
                log("SOCKS5 destination name too long", level: "ERROR")
                closeRelay(clientFlow, proxyConnection, error: FlowFailure.refused)
                return
            }
            request.append(0x03)
            request.append(UInt8(hostBytes.count))
            request.append(contentsOf: hostBytes)
        }
        
        request.append(UInt8(port >> 8))
        request.append(UInt8(port & 0xFF))
        
        proxyConnection.write(request) { [weak self] error in
            if let error = error {
                self?.log("SOCKS5 connect write failed: \(error.localizedDescription)", level: "ERROR")
                self?.closeRelay(clientFlow, proxyConnection, error: FlowFailure.proxyUnreachable)
                return
            }
            
            self?.readSOCKS5Reply(proxyConnection) { [weak self] reply in
                guard let self = self else { return }
                switch reply {
                case .success:
                    self.relayData(clientFlow: clientFlow, proxyConnection: proxyConnection)
                case .rejected(let rep):
                    self.log("SOCKS5 connect rejected, REP=0x\(String(format: "%02x", rep)) (\(self.socksReplyReason(rep)))", level: "ERROR")
                    self.closeRelay(clientFlow, proxyConnection, error: FlowFailure.socks(rep))
                case .failed(let reason):
                    self.log("SOCKS5 connect failed: \(reason)", level: "ERROR")
                    self.closeRelay(clientFlow, proxyConnection, error: FlowFailure.proxyUnreachable)
                }
            }
        }
    }
    
    private func handleHTTPProxy(clientFlow: NEAppProxyTCPFlow, proxyConnection: NWTCPConnection, destination: String, port: UInt16, username: String?, password: String?) {
        // ipv6 literals must be bracketed in the request line and Host header
        let host = IPv6Address(destination) != nil ? "[\(destination)]" : destination
        var request = "CONNECT \(host):\(port) HTTP/1.1\r\n"
        request += "Host: \(host):\(port)\r\n"
        
        if let username = username, let password = password {
            let credentials = "\(username):\(password)"
            if let credData = credentials.data(using: .utf8) {
                let base64Creds = credData.base64EncodedString()
                request += "Proxy-Authorization: Basic \(base64Creds)\r\n"
            }
        }
        
        request += "\r\n"
        
        guard let requestData = request.data(using: .utf8) else {
            log("HTTP CONNECT request encoding failed", level: "ERROR")
            closeRelay(clientFlow, proxyConnection, error: FlowFailure.refused)
            return
        }
        
        proxyConnection.write(requestData) { [weak self] error in
            if let error = error {
                self?.log("HTTP CONNECT write failed: \(error.localizedDescription)", level: "ERROR")
                self?.closeRelay(clientFlow, proxyConnection, error: FlowFailure.proxyUnreachable)
                return
            }
            
            self?.readHTTPConnectResponse(proxyConnection) { [weak self] status, leftover, detail in
                guard let self = self else { return }
                // any 2xx opens the tunnel, a "200" somewhere else in the reply
                // (407 with Content-Length: 200) used to count as success
                if let status = status, (200...299).contains(status) {
                    self.relayData(clientFlow: clientFlow, proxyConnection: proxyConnection, initialData: leftover)
                } else {
                    self.log("HTTP CONNECT failed: \(detail)", level: "ERROR")
                    self.closeRelay(clientFlow, proxyConnection, error: FlowFailure.http(status))
                }
            }
        }
    }
    
    private func relayData(clientFlow: NEAppProxyTCPFlow, proxyConnection: NWTCPConnection, initialData: Data? = nil) {
        guard completeHandshake(proxyConnection) else { return }
        clientFlow.open(withLocalEndpoint: nil) { [weak self] error in
            if let error = error {
                self?.log("Failed to open client flow: \(error.localizedDescription)", level: "ERROR")
                self?.closeRelay(clientFlow, proxyConnection, error: error)
                return
            }
            
            self?.relayClientToProxy(clientFlow: clientFlow, proxyConnection: proxyConnection)
            // bytes the proxy sent right behind its reply are the start of the
            // server's data, deliver them before the normal relay takes over
            if let initial = initialData, !initial.isEmpty {
                self?.addFlowBytes(clientFlow, sentDelta: 0, recvDelta: Int64(initial.count))
                clientFlow.write(initial) { [weak self] error in
                    if let error = error {
                        self?.log("Client write error: \(error.localizedDescription)", level: "ERROR")
                        self?.closeRelay(clientFlow, proxyConnection, error: error)
                        return
                    }
                    self?.relayProxyToClient(clientFlow: clientFlow, proxyConnection: proxyConnection)
                }
            } else {
                self?.relayProxyToClient(clientFlow: clientFlow, proxyConnection: proxyConnection)
            }
        }
    }
    
    private func relayClientToProxy(clientFlow: NEAppProxyTCPFlow, proxyConnection: NWTCPConnection) {
        clientFlow.readData { [weak self] data, error in
            if let error = error {
                // ignore expected errors
                let code = (error as NSError).code
                if code != 57 && code != 54 && code != 89 {
                    self?.log("Client read error: \(error.localizedDescription)", level: "ERROR")
                }
                self?.closeRelay(clientFlow, proxyConnection)
                return
            }
            
            guard let data = data, !data.isEmpty else {
                // the client finished sending (FIN). macOS closes the flow in
                // both directions at that point (the system log shows "Closing
                // writes" right after the EOF), so a reply can't be delivered
                // to a client that half-closes. send a FIN upstream so the request
                // is flushed cleanly, then tear down after a short grace instead
                // of cancelling mid-flush.
                proxyConnection.writeClose()
                DispatchQueue.global(qos: .utility).asyncAfter(deadline: .now() + AppProxyProvider.halfCloseTimeout) { [weak self] in
                    self?.closeRelay(clientFlow, proxyConnection)
                }
                return
            }

            self?.addFlowBytes(clientFlow, sentDelta: Int64(data.count), recvDelta: 0)
            proxyConnection.write(data) { error in
                if let error = error {
                    self?.log("Proxy write error: \(error.localizedDescription)", level: "ERROR")
                    self?.closeRelay(clientFlow, proxyConnection, error: error)
                } else {
                    self?.relayClientToProxy(clientFlow: clientFlow, proxyConnection: proxyConnection)
                }
            }
        }
    }
    
    private func relayProxyToClient(clientFlow: NEAppProxyTCPFlow, proxyConnection: NWTCPConnection) {
        proxyConnection.readMinimumLength(1, maximumLength: 65536) { [weak self] data, error in
            if let error = error {
                let code = (error as NSError).code
                if code != 57 && code != 54 && code != 89 {
                    self?.log("Proxy read error: \(error.localizedDescription)", level: "ERROR")
                }
                self?.closeRelay(clientFlow, proxyConnection)
                return
            }
            
            guard let data = data, !data.isEmpty else {
                self?.closeRelay(clientFlow, proxyConnection)
                return
            }
            
            self?.addFlowBytes(clientFlow, sentDelta: 0, recvDelta: Int64(data.count))
            clientFlow.write(data) { error in
                if let error = error {
                    self?.log("Client write error: \(error.localizedDescription)", level: "ERROR")
                    self?.closeRelay(clientFlow, proxyConnection, error: error)
                } else {
                    self?.relayProxyToClient(clientFlow: clientFlow, proxyConnection: proxyConnection)
                }
            }
        }
    }
    
    private func findMatchingRule(bundleId: String, processName: String?, executablePath: String? = nil, ancestorPaths: [String] = [], destination: String, port: UInt16, connectionProtocol: RuleProtocol, checkIpPort: Bool, domains: [String] = []) -> ProxyRule? {
        rulesLock.lock()
        let currentRules = rules
        rulesLock.unlock()
        if currentRules.isEmpty { return nil }

        let flowProcess = ProxyRule.FlowProcess(bundleId: bundleId, processName: processName, executablePath: executablePath, ancestorPaths: ancestorPaths)
        let domainChars = domains.map { (name: $0, chars: Array($0)) }

        var excludedBy: ProxyRule? = nil

        for (rule, m) in currentRules {
            guard rule.enabled else { continue }

            if rule.ruleProtocol != .both && rule.ruleProtocol != connectionProtocol {
                continue
            }

            if checkIpPort {
                if !m.matchesPort(port) || !m.matchesHost(ip: destination, domains: domainChars) {
                    continue
                }
            } else if m.hasHostFilter || m.hasPortFilter {
                // UDP flow has no initial destination here, so any host/port-filtered rule cannot match
                continue
            }

            if rule.excludeProcess {
                if m.matchesProcess(flowProcess) {
                    // Process is explicitly excluded from this rule: allow later rules to match it,
                    // but if none do, keep it DIRECT instead of falling back to defaultProxyId.
                    if rule.action != "DIRECT" && rule.action != "BLOCK" && excludedBy == nil {
                        excludedBy = rule
                    }
                    continue
                }
                return rule
            }

            if m.matchesProcess(flowProcess) {
                return rule
            }
        }

        if let ex = excludedBy {
            return ProxyRule(
                ruleId: ex.ruleId,
                processNames: ex.processNames,
                targetHosts: ex.targetHosts,
                targetPorts: ex.targetPorts,
                ruleProtocol: ex.ruleProtocol,
                action: "DIRECT",
                enabled: true,
                excludeProcess: false
            )
        }
        return nil
    }
    
    private static func isLoopbackAddress(_ ip: String) -> Bool {
        let lower = ip.lowercased()
        return lower == "localhost" || lower == "::1" || lower.hasPrefix("127.")
    }

    private static func isNonRoutableNonLoopback(_ ip: String) -> Bool {
        let lower = ip.lowercased()
        if lower == "::" || lower == "0.0.0.0" || lower == "255.255.255.255" {
            return true
        }
        if lower.hasPrefix("0.") || lower.hasPrefix("169.254.") || lower.hasPrefix("fe80:") || lower.hasPrefix("fec0:") || (lower.hasPrefix("ff") && lower.contains(":")) || lower.hasSuffix(".255") {
            return true
        }
        if let firstDot = lower.firstIndex(of: "."), let firstOctet = Int(lower[..<firstDot]), firstOctet >= 224 && firstOctet <= 239 {
            return true
        }
        return false
    }

    private static func isLocalOrMulticastAddress(_ ip: String) -> Bool {
        return isLoopbackAddress(ip) || isNonRoutableNonLoopback(ip)
    }

    private func sendLogToApp(protocol: String, pid: Int = 0, process: String, destination: String, port: String, proxy: String) {
        guard trafficLoggingEnabled else { return }
        appendLog(.connection(proto: `protocol`, pid: pid, process: process, destination: destination, port: port, proxy: proxy))
    }
}


