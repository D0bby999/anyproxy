// SPDX-License-Identifier: AGPL-3.0-or-later

package pbprofile

import (
	"bytes"
	"os"
	"strings"
	"testing"
)

func TestParseMacPBTestProfile(t *testing.T) {
	f, err := os.Open("/tmp/ProxyBridge/MacOS/tests/pbtest.pbprofile")
	if err != nil {
		t.Skipf("upstream fixture not present: %v", err)
	}
	defer f.Close()
	prof, err := ParseProfile(f)
	if err != nil {
		t.Fatalf("ParseProfile(pbtest.pbprofile): %v", err)
	}
	if prof.Name != "PBTest" {
		t.Fatalf("Name = %q, want PBTest", prof.Name)
	}
	if len(prof.Proxies) != 5 {
		t.Fatalf("len(Proxies) = %d, want 5", len(prof.Proxies))
	}
	if len(prof.Rules) != 10 {
		t.Fatalf("len(Rules) = %d, want 10", len(prof.Rules))
	}
	if prof.Rules[0].Action != ActionProxy || prof.Rules[0].ProxyConfigID != "pbtest-socks-1080" {
		t.Fatalf("rule 0 = %+v", prof.Rules[0])
	}
	if prof.Rules[0].TargetDomains != "example.com" || prof.Rules[0].TargetHosts != "*" {
		t.Fatalf("rule 0 hosts/domains = %q / %q", prof.Rules[0].TargetHosts, prof.Rules[0].TargetDomains)
	}
	if prof.Rules[9].TargetHosts != "192.0.2.20;192.0.2.21;192.0.2.31" || prof.Rules[9].TargetDomains != "" {
		t.Fatalf("rule 9 hosts/domains = %q / %q", prof.Rules[9].TargetHosts, prof.Rules[9].TargetDomains)
	}

	// Round-trip marshal & re-parse
	marshaled, err := MarshalProfile(prof)
	if err != nil {
		t.Fatalf("MarshalProfile: %v", err)
	}
	reparsed, err := ParseProfile(bytes.NewReader(marshaled))
	if err != nil {
		t.Fatalf("re-parse: %v", err)
	}
	if len(reparsed.Proxies) != 5 || len(reparsed.Rules) != 10 {
		t.Fatalf("reparsed counts = %d, %d", len(reparsed.Proxies), len(reparsed.Rules))
	}
	if reparsed.Rules[0].CombinedHosts() != "example.com" {
		t.Fatalf("reparsed rule 0 CombinedHosts = %q", reparsed.Rules[0].CombinedHosts())
	}
}

func TestParseWindowsLinuxProfile(t *testing.T) {
	winJSON := `{
  "Version": "1.0",
  "Name": "work",
  "IsTrafficLoggingEnabled": true,
  "DnsViaProxy": true,
  "DefaultProxyId": 1,
  "ProxyConfigs": [
    { "Id": 1, "Name": "corp", "Type": "SOCKS5", "Host": "proxy.corp", "Port": "1080", "Username": "", "Password": "", "SendDomainToProxy": true },
    { "Id": 2, "Name": "web",  "Type": "HTTP",   "Host": "10.0.0.2",   "Port": 8080,   "Username": "bob", "Password": "secret" }
  ],
  "ProxyRules": [
    { "Name": "Browsers", "ProcessName": "firefox;chrome", "TargetHosts": "10.0.0.0/8", "TargetPorts": "80;443", "TargetDomains": "*.github.com",
      "Protocol": "TCP", "Action": "PROXY", "IsEnabled": true, "ProxyConfigId": 2, "LocalhostViaProxy": true }
  ]
}`
	prof, err := ParseProfile(strings.NewReader(winJSON))
	if err != nil {
		t.Fatalf("ParseProfile: %v", err)
	}
	if prof.Name != "work" || len(prof.Proxies) != 2 || len(prof.Rules) != 1 {
		t.Fatalf("unexpected profile: %+v", prof)
	}
	r := prof.Rules[0]
	if r.ProxyConfigID != "proxy-2" || !r.LocalhostViaProxy || r.CombinedHosts() != "10.0.0.0/8;*.github.com" {
		t.Fatalf("unexpected rule: %+v (combined=%q)", r, r.CombinedHosts())
	}
}

func TestRejectControlChars(t *testing.T) {
	bad := `{"name":"bad\u0007name","proxyConfigs":[],"proxyRules":[]}`
	if _, err := ParseProfile(strings.NewReader(bad)); err == nil {
		t.Fatal("expected error for control char in profile name")
	}
}

func TestRulesJSONRoundTrip(t *testing.T) {
	raw := `[
  {
    "action": "DIRECT",
    "enabled": true,
    "processNames": "curl",
    "excludeProcess": true,
    "protocol": "BOTH",
    "targetHosts": "192.168.*.*;*.github.com",
    "targetPorts": "80;443"
  }
]`
	rules, err := ParseRulesJSON(strings.NewReader(raw), "")
	if err != nil {
		t.Fatalf("ParseRulesJSON: %v", err)
	}
	if len(rules) != 1 || !rules[0].ExcludeProcess || rules[0].TargetHosts != "192.168.*.*" || rules[0].TargetDomains != "*.github.com" {
		t.Fatalf("unexpected parsed rule: %+v", rules[0])
	}
	b, err := MarshalRulesJSON(rules)
	if err != nil {
		t.Fatalf("MarshalRulesJSON: %v", err)
	}
	rules2, err := ParseRulesJSON(bytes.NewReader(b), "")
	if err != nil || len(rules2) != 1 || !rules2[0].ExcludeProcess || rules2[0].CombinedHosts() != "192.168.*.*;*.github.com" {
		t.Fatalf("round-trip failed: %+v, %v", rules2, err)
	}
}

func TestHTTPSProxyRoundTripAndValidation(t *testing.T) {
	raw := `{
  "Version": "1.0",
  "Name": "https-profile",
  "DefaultProxyId": 1,
  "ProxyConfigs": [
    { "Id": 1, "Name": "edge-tls", "Type": "HTTPS", "Host": "proxy.example.com", "Port": 443, "Username": "u", "Password": "p", "SendDomainToProxy": true }
  ],
  "ProxyRules": []
}`
	prof, err := ParseProfile(strings.NewReader(raw))
	if err != nil {
		t.Fatalf("ParseProfile(HTTPS): %v", err)
	}
	if len(prof.Proxies) != 1 || prof.Proxies[0].Type != ProxyHTTPS {
		t.Fatalf("expected ProxyHTTPS, got %+v", prof.Proxies)
	}
	b, err := MarshalProfile(prof)
	if err != nil {
		t.Fatalf("MarshalProfile: %v", err)
	}
	prof2, err := ParseProfile(bytes.NewReader(b))
	if err != nil {
		t.Fatalf("re-parse HTTPS profile: %v", err)
	}
	if len(prof2.Proxies) != 1 || prof2.Proxies[0].Type != ProxyHTTPS {
		t.Fatalf("expected ProxyHTTPS after round-trip, got %+v", prof2.Proxies)
	}

	invalid := Profile{
		Name: "bad",
		Proxies: []ProxyConfig{{
			ID: "p1", Name: "bad", Type: ProxyType("SOCKS4"), Host: "127.0.0.1", Port: 1080,
		}},
	}
	if err := ValidateProfile(&invalid); err == nil {
		t.Fatal("expected validation error for unsupported proxy type SOCKS4")
	}
}
