package utils

import "testing"

func TestNormalizeAuthAccessConfigNormalizesEntries(t *testing.T) {
	config, err := NormalizeAuthAccessConfig(AuthAccessConfig{
		AccessRules: AuthAccessRulesConfig{
			BypassIPs:        []string{" 127.0.0.1 ", "127.0.0.1", "192.168.1.0/24"},
			BypassDomains:    []string{"LOCALHOST", "*.Trusted.Example.com", "https://admin.example.com/path"},
			ForceAuthIPs:     []string{"203.0.113.9"},
			ForceAuthDomains: []string{"secure.example.com:3007"},
		},
		ProxyHeader:    "  X-Real-IP  ",
		TrustedProxies: []string{"10.0.0.0/24", "10.0.0.1"},
	})
	if err != nil {
		t.Fatalf("NormalizeAuthAccessConfig returned error: %v", err)
	}

	if got, want := config.ProxyHeader, DefaultAuthProxyHeader; got != want {
		t.Fatalf("ProxyHeader = %q, want %q", got, want)
	}
	if got, want := config.AccessRules.BypassIPs, []string{"127.0.0.1", "192.168.1.0/24"}; !stringSlicesEqual(got, want) {
		t.Fatalf("BypassIPs = %#v, want %#v", got, want)
	}
	if got, want := config.AccessRules.BypassDomains, []string{"localhost", "*.trusted.example.com", "admin.example.com"}; !stringSlicesEqual(got, want) {
		t.Fatalf("BypassDomains = %#v, want %#v", got, want)
	}
	if got, want := config.AccessRules.ForceAuthDomains, []string{"secure.example.com"}; !stringSlicesEqual(got, want) {
		t.Fatalf("ForceAuthDomains = %#v, want %#v", got, want)
	}
}

func TestNormalizeAuthAccessConfigRejectsInvalidEntries(t *testing.T) {
	_, err := NormalizeAuthAccessConfig(AuthAccessConfig{
		AccessRules: AuthAccessRulesConfig{
			BypassIPs: []string{"not-an-ip"},
		},
	})
	if err == nil {
		t.Fatal("expected invalid bypass IP rule to fail validation")
	}
}

func TestMatchAuthAccessRulesSupportsCIDRDomainsAndForceOverride(t *testing.T) {
	rules := AuthAccessRulesConfig{
		BypassIPs:        []string{"192.168.1.0/24"},
		BypassDomains:    []string{"localhost", "*.trusted.example.com"},
		ForceAuthIPs:     []string{"192.168.1.20"},
		ForceAuthDomains: []string{"admin.example.com"},
	}

	if match := MatchAuthAccessRules(rules, "192.168.1.18", "service.internal"); !match.Bypassed || match.ForceAuth {
		t.Fatalf("expected CIDR bypass match, got %#v", match)
	}

	if match := MatchAuthAccessRules(rules, "203.0.113.8", "api.trusted.example.com:3007"); !match.Bypassed || match.ForceAuth {
		t.Fatalf("expected wildcard host bypass match, got %#v", match)
	}

	if match := MatchAuthAccessRules(rules, "203.0.113.8", "trusted.example.com"); match.Bypassed {
		t.Fatalf("expected wildcard host not to match root domain, got %#v", match)
	}

	if match := MatchAuthAccessRules(rules, "192.168.1.20", "localhost"); !match.ForceAuth || match.Bypassed {
		t.Fatalf("expected forceAuth IP to override bypass, got %#v", match)
	}

	if match := MatchAuthAccessRules(rules, "203.0.113.8", "admin.example.com"); !match.ForceAuth || match.Bypassed {
		t.Fatalf("expected forceAuth host to win, got %#v", match)
	}
}

func TestIsTrustedProxySupportsExactIPsAndCIDR(t *testing.T) {
	if !IsTrustedProxy("10.0.0.5", []string{"10.0.0.0/24"}) {
		t.Fatal("expected proxy CIDR to match")
	}
	if !IsTrustedProxy("127.0.0.1", []string{"127.0.0.1"}) {
		t.Fatal("expected exact proxy IP to match")
	}
	if IsTrustedProxy("192.0.2.10", []string{"10.0.0.0/24"}) {
		t.Fatal("expected unrelated IP not to match")
	}
}

func stringSlicesEqual(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}
