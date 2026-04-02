package main

import "testing"

func TestParseUFWNumberedPortRuleIPv4(t *testing.T) {
	line := "[ 4] 22/tcp                     ALLOW IN    Anywhere                   # SSH"
	rule, ok := parseUFWNumberedPortRule(line)
	if !ok {
		t.Fatalf("expected parser to accept ipv4 ufw port rule")
	}
	if rule.Number != 4 || rule.Port != 22 || rule.Protocol != "tcp" || rule.Block || rule.IPv6 {
		t.Fatalf("unexpected parse result: %+v", rule)
	}
	if rule.Reason != "SSH" {
		t.Fatalf("expected reason to be parsed, got %q", rule.Reason)
	}
}

func TestParseUFWNumberedPortRuleIPv6(t *testing.T) {
	line := "[ 5] 22/tcp (v6)                ALLOW IN    Anywhere (v6)              # SSH"
	rule, ok := parseUFWNumberedPortRule(line)
	if !ok {
		t.Fatalf("expected parser to accept ipv6 ufw port rule")
	}
	if rule.Number != 5 || rule.Port != 22 || rule.Protocol != "tcp" || rule.Block || !rule.IPv6 {
		t.Fatalf("unexpected parse result: %+v", rule)
	}
}

func TestParseUFWNumberedPortRuleRejectsNonAnywhereSource(t *testing.T) {
	line := "[ 7] 443/tcp                    ALLOW IN    198.51.100.10"
	if _, ok := parseUFWNumberedPortRule(line); ok {
		t.Fatalf("expected parser to skip non-anywhere source port rule")
	}
}
