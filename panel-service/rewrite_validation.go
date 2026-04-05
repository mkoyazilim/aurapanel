package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func validateWebsiteRewriteRules(domain, rules string) error {
	return validateRewriteRulesForDocroot(domainDocroot(domain), rules)
}

func validateRewriteRulesForDocroot(docroot, rules string) error {
	rules = strings.TrimSpace(rules)
	if rules == "" {
		return nil
	}

	lines := strings.Split(rules, "\n")
	for i, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		pattern, target, ok := parseRewriteRuleLine(line)
		if !ok {
			continue
		}
		if err := validateRewriteTarget(docroot, pattern, target); err != nil {
			return fmt.Errorf("rewrite satiri %d gecersiz: %w", i+1, err)
		}
	}
	return nil
}

func parseRewriteRuleLine(line string) (string, string, bool) {
	fields := strings.Fields(line)
	if len(fields) < 3 {
		return "", "", false
	}
	if !strings.EqualFold(fields[0], "RewriteRule") {
		return "", "", false
	}
	return strings.TrimSpace(fields[1]), strings.TrimSpace(fields[2]), true
}

func validateRewriteTarget(docroot, pattern, target string) error {
	target = strings.Trim(target, `"'`)
	if target == "" || target == "-" {
		return nil
	}
	lower := strings.ToLower(target)
	if strings.HasPrefix(lower, "http://") || strings.HasPrefix(lower, "https://") || strings.HasPrefix(lower, "ftp://") {
		return nil
	}
	if strings.HasPrefix(target, "/") || strings.HasPrefix(target, "%{") {
		return nil
	}

	pathPart := target
	if idx := strings.IndexAny(pathPart, "?#"); idx >= 0 {
		pathPart = pathPart[:idx]
	}
	pathPart = strings.TrimSpace(strings.Trim(pathPart, `"'`))
	if pathPart == "" {
		return nil
	}

	prefix := pathPart
	if idx := strings.IndexAny(prefix, "$%"); idx >= 0 {
		prefix = prefix[:idx]
	}
	prefix = strings.TrimSpace(prefix)
	prefix = strings.TrimPrefix(prefix, "./")
	prefix = strings.TrimPrefix(prefix, "/")
	if prefix == "" {
		return nil
	}

	clean := filepath.Clean(filepath.FromSlash(prefix))
	if clean == "." {
		return nil
	}
	targetPath := filepath.Join(docroot, clean)
	info, err := os.Stat(targetPath)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("rewrite hedefi bulunamadi: %s", filepath.ToSlash(clean))
		}
		return err
	}
	if !info.IsDir() {
		return nil
	}
	if !isCatchAllRewritePattern(pattern) {
		return nil
	}
	if fileExists(filepath.Join(targetPath, "index.php")) || fileExists(filepath.Join(targetPath, "index.html")) {
		return nil
	}
	return fmt.Errorf("rewrite hedef klasorunde index yok: %s", filepath.ToSlash(clean))
}

func isCatchAllRewritePattern(pattern string) bool {
	pattern = strings.ReplaceAll(strings.TrimSpace(pattern), " ", "")
	switch pattern {
	case "^(.*)$", "^(.+)$", "^.*$", "^.+$":
		return true
	default:
		return false
	}
}
