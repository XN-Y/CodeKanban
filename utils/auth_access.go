package utils

import (
	"fmt"
	"net"
	"net/netip"
	"net/url"
	"strconv"
	"strings"
)

const DefaultAuthProxyHeader = "X-Forwarded-For"

type AuthAccessRulesConfig struct {
	BypassIPs        []string `json:"bypassIPs" yaml:"bypassIPs"`
	BypassDomains    []string `json:"bypassDomains" yaml:"bypassDomains"`
	ForceAuthIPs     []string `json:"forceAuthIPs" yaml:"forceAuthIPs"`
	ForceAuthDomains []string `json:"forceAuthDomains" yaml:"forceAuthDomains"`
}

type AuthAccessConfig struct {
	AccessRules    AuthAccessRulesConfig `json:"accessRules" yaml:"accessRules"`
	ProxyHeader    string                `json:"proxyHeader" yaml:"proxyHeader"`
	TrustedProxies []string              `json:"trustedProxies" yaml:"trustedProxies"`
}

type AuthAccessMatch struct {
	Bypassed  bool
	ForceAuth bool
}

func DefaultAuthAccessConfig() AuthAccessConfig {
	return AuthAccessConfig{
		AccessRules: AuthAccessRulesConfig{
			BypassIPs:        []string{},
			BypassDomains:    []string{},
			ForceAuthIPs:     []string{},
			ForceAuthDomains: []string{},
		},
		ProxyHeader:    DefaultAuthProxyHeader,
		TrustedProxies: []string{},
	}
}

func AuthAccessConfigFromAuthConfig(config AuthConfig) AuthAccessConfig {
	return AuthAccessConfig{
		AccessRules:    config.AccessRules,
		ProxyHeader:    config.ProxyHeader,
		TrustedProxies: config.TrustedProxies,
	}
}

func ApplyAuthAccessConfigToAuthConfig(config *AuthConfig, access AuthAccessConfig) {
	if config == nil {
		return
	}
	config.AccessRules = access.AccessRules
	config.ProxyHeader = access.ProxyHeader
	config.TrustedProxies = access.TrustedProxies
}

func SanitizeAuthConfig(config AuthConfig) AuthConfig {
	sanitized := config
	ApplyAuthAccessConfigToAuthConfig(&sanitized, SanitizeAuthAccessConfig(AuthAccessConfigFromAuthConfig(config)))
	return sanitized
}

func SanitizeAuthAccessConfig(config AuthAccessConfig) AuthAccessConfig {
	normalized := DefaultAuthAccessConfig()
	normalized.AccessRules = sanitizeAuthAccessRulesConfig(config.AccessRules)
	normalized.TrustedProxies = sanitizeIPRuleList(config.TrustedProxies)
	return normalized
}

func NormalizeAuthAccessConfig(config AuthAccessConfig) (AuthAccessConfig, error) {
	normalized := DefaultAuthAccessConfig()

	accessRules, err := NormalizeAuthAccessRulesConfig(config.AccessRules)
	if err != nil {
		return normalized, err
	}
	normalized.AccessRules = accessRules

	trustedProxies, err := normalizeIPRuleList(config.TrustedProxies, "trustedProxies", true)
	if err != nil {
		return normalized, err
	}
	normalized.TrustedProxies = trustedProxies
	return normalized, nil
}

func NormalizeAuthAccessRulesConfig(config AuthAccessRulesConfig) (AuthAccessRulesConfig, error) {
	normalized := AuthAccessRulesConfig{}

	var err error
	if normalized.BypassIPs, err = normalizeIPRuleList(config.BypassIPs, "accessRules.bypassIPs", true); err != nil {
		return normalized, err
	}
	if normalized.BypassDomains, err = normalizeHostRuleList(config.BypassDomains, "accessRules.bypassDomains", true); err != nil {
		return normalized, err
	}
	if normalized.ForceAuthIPs, err = normalizeIPRuleList(config.ForceAuthIPs, "accessRules.forceAuthIPs", true); err != nil {
		return normalized, err
	}
	if normalized.ForceAuthDomains, err = normalizeHostRuleList(config.ForceAuthDomains, "accessRules.forceAuthDomains", true); err != nil {
		return normalized, err
	}
	return normalized, nil
}

func MatchAuthAccessRules(rules AuthAccessRulesConfig, clientIP string, host string) AuthAccessMatch {
	sanitized := sanitizeAuthAccessRulesConfig(rules)
	normalizedIP := normalizeRequestIP(clientIP)
	normalizedHost := normalizeRequestHost(host)

	match := AuthAccessMatch{
		ForceAuth: matchIPRules(normalizedIP, sanitized.ForceAuthIPs) ||
			matchHostRules(normalizedHost, sanitized.ForceAuthDomains),
		Bypassed: matchIPRules(normalizedIP, sanitized.BypassIPs) ||
			matchHostRules(normalizedHost, sanitized.BypassDomains),
	}
	if match.ForceAuth {
		match.Bypassed = false
	}
	return match
}

func IsTrustedProxy(remoteIP string, trustedProxies []string) bool {
	normalizedIP := normalizeRequestIP(remoteIP)
	if normalizedIP == "" {
		return false
	}
	return matchIPRules(normalizedIP, sanitizeIPRuleList(trustedProxies))
}

func sanitizeAuthAccessRulesConfig(config AuthAccessRulesConfig) AuthAccessRulesConfig {
	return AuthAccessRulesConfig{
		BypassIPs:        sanitizeIPRuleList(config.BypassIPs),
		BypassDomains:    sanitizeHostRuleList(config.BypassDomains),
		ForceAuthIPs:     sanitizeIPRuleList(config.ForceAuthIPs),
		ForceAuthDomains: sanitizeHostRuleList(config.ForceAuthDomains),
	}
}

func sanitizeIPRuleList(values []string) []string {
	items, _ := normalizeIPRuleList(values, "", false)
	return items
}

func sanitizeHostRuleList(values []string) []string {
	items, _ := normalizeHostRuleList(values, "", false)
	return items
}

func normalizeIPRuleList(values []string, field string, strict bool) ([]string, error) {
	normalized := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))

	for index, raw := range values {
		value := strings.TrimSpace(raw)
		if value == "" {
			continue
		}

		item, err := normalizeIPRule(value)
		if err != nil {
			if strict {
				return nil, fmt.Errorf("%s[%d] must be a valid IP or CIDR", field, index)
			}
			continue
		}
		if _, exists := seen[item]; exists {
			continue
		}
		seen[item] = struct{}{}
		normalized = append(normalized, item)
	}

	return normalized, nil
}

func normalizeHostRuleList(values []string, field string, strict bool) ([]string, error) {
	normalized := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))

	for index, raw := range values {
		value := strings.TrimSpace(raw)
		if value == "" {
			continue
		}

		item, err := normalizeHostRule(value)
		if err != nil {
			if strict {
				return nil, fmt.Errorf("%s[%d] must be an exact host or *.subdomain pattern", field, index)
			}
			continue
		}
		if _, exists := seen[item]; exists {
			continue
		}
		seen[item] = struct{}{}
		normalized = append(normalized, item)
	}

	return normalized, nil
}

func normalizeIPRule(value string) (string, error) {
	if prefix, err := netip.ParsePrefix(value); err == nil {
		return prefix.Masked().String(), nil
	}
	addr, err := netip.ParseAddr(value)
	if err != nil {
		return "", err
	}
	return addr.Unmap().String(), nil
}

func normalizeHostRule(value string) (string, error) {
	pattern := strings.TrimSpace(value)
	wildcard := strings.HasPrefix(pattern, "*.")
	if wildcard {
		pattern = strings.TrimPrefix(pattern, "*.")
	}

	host, err := normalizeHostToken(pattern)
	if err != nil {
		return "", err
	}
	if host == "" || strings.Contains(host, "*") || strings.Contains(host, "/") {
		return "", fmt.Errorf("invalid host pattern")
	}
	if wildcard {
		if net.ParseIP(host) != nil {
			return "", fmt.Errorf("wildcard host cannot be an IP")
		}
		return "*." + host, nil
	}
	return host, nil
}

func normalizeRequestIP(value string) string {
	addr, err := netip.ParseAddr(strings.TrimSpace(value))
	if err != nil {
		return ""
	}
	return addr.Unmap().String()
}

func normalizeRequestHost(value string) string {
	host, err := normalizeHostToken(value)
	if err != nil {
		return ""
	}
	return host
}

func normalizeHostToken(value string) (string, error) {
	host := strings.TrimSpace(value)
	if host == "" {
		return "", fmt.Errorf("empty host")
	}

	if parsed, err := url.Parse(host); err == nil && parsed.Host != "" {
		host = parsed.Host
	}

	host = strings.TrimSpace(strings.Split(host, ",")[0])
	if host == "" {
		return "", fmt.Errorf("empty host")
	}

	if parsedHost, _, err := net.SplitHostPort(host); err == nil {
		host = parsedHost
	} else if strings.Count(host, ":") == 1 && !strings.Contains(host, "]") {
		lastColon := strings.LastIndex(host, ":")
		if lastColon > 0 {
			if _, portErr := strconv.Atoi(host[lastColon+1:]); portErr == nil {
				host = host[:lastColon]
			}
		}
	}

	host = strings.TrimPrefix(host, "[")
	host = strings.TrimSuffix(host, "]")
	host = strings.TrimSuffix(host, ".")
	host = strings.ToLower(strings.TrimSpace(host))
	if host == "" || strings.ContainsAny(host, " /") {
		return "", fmt.Errorf("invalid host")
	}
	return host, nil
}

func matchIPRules(clientIP string, rules []string) bool {
	if clientIP == "" {
		return false
	}

	addr, err := netip.ParseAddr(clientIP)
	if err != nil {
		return false
	}
	addr = addr.Unmap()

	for _, rule := range rules {
		if strings.Contains(rule, "/") {
			prefix, err := netip.ParsePrefix(rule)
			if err == nil && prefix.Contains(addr) {
				return true
			}
			continue
		}

		ruleAddr, err := netip.ParseAddr(rule)
		if err == nil && ruleAddr.Unmap() == addr {
			return true
		}
	}

	return false
}

func matchHostRules(host string, rules []string) bool {
	if host == "" {
		return false
	}

	for _, rule := range rules {
		if strings.HasPrefix(rule, "*.") {
			suffix := strings.TrimPrefix(rule, "*.")
			if host != suffix && strings.HasSuffix(host, "."+suffix) {
				return true
			}
			continue
		}
		if host == rule {
			return true
		}
	}

	return false
}
