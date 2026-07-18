// Package platformconfig reads the optional platform.yaml that lives in an
// app repo alongside the code. It supplies the hints the detector can't
// infer on its own: replica count, resource sizing, env vars, and whether
// the service should be reachable outside the cluster.
//
// This intentionally implements only the small, flat subset of YAML the
// platform's schema needs (scalars, one level of nested `resources:` and
// one list `env:`), rather than pulling in a general YAML library. If the
// file is absent, sane defaults are used — platform.yaml is an override,
// never a requirement.
package platformconfig

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type EnvVar struct {
	Name      string
	Value     string
	SecretRef string // if set, sourced from a K8s Secret instead of a literal value
}

type Resources struct {
	CPURequest    string
	MemoryRequest string
	CPULimit      string
	MemoryLimit   string
}

type Config struct {
	Port      int
	Replicas  int
	Public    bool // expose via Ingress
	Env       []EnvVar
	Resources Resources
}

// Default returns the platform's baseline resource sizing — deliberately
// conservative, so an app with no platform.yaml still gets safe limits
// rather than "no limits at all".
func Default(detectedPort int) Config {
	return Config{
		Port:     detectedPort,
		Replicas: 2,
		Public:   false,
		Resources: Resources{
			CPURequest:    "100m",
			MemoryRequest: "128Mi",
			CPULimit:      "500m",
			MemoryLimit:   "512Mi",
		},
	}
}

// Load reads platform.yaml from repoPath if present, layering it on top of
// the detector-derived defaults. Returns defaults unchanged if no file exists.
func Load(repoPath string, detectedPort int) (Config, error) {
	cfg := Default(detectedPort)
	path := filepath.Join(repoPath, "platform.yaml")

	f, err := os.Open(path)
	if os.IsNotExist(err) {
		return cfg, nil
	} else if err != nil {
		return cfg, fmt.Errorf("opening platform.yaml: %w", err)
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	inEnvList := false
	inResources := false

	for scanner.Scan() {
		raw := scanner.Text()
		line := strings.TrimRight(raw, " \t")
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		indent := len(line) - len(strings.TrimLeft(line, " "))

		// List items under `env:`
		if strings.HasPrefix(trimmed, "- ") && inEnvList {
			item := strings.TrimPrefix(trimmed, "- ")
			ev, err := parseEnvItem(item)
			if err != nil {
				return cfg, fmt.Errorf("parsing env item %q: %w", item, err)
			}
			cfg.Env = append(cfg.Env, ev)
			continue
		}

		if indent == 0 {
			inEnvList = false
			inResources = false
		}

		key, val, ok := splitKV(trimmed)
		if !ok {
			continue
		}

		switch {
		case indent == 0 && key == "port":
			cfg.Port = atoiOr(val, cfg.Port)
		case indent == 0 && key == "replicas":
			cfg.Replicas = atoiOr(val, cfg.Replicas)
		case indent == 0 && key == "public":
			cfg.Public = val == "true"
		case indent == 0 && key == "env":
			inEnvList = true
		case indent == 0 && key == "resources":
			inResources = true
		case inResources && key == "cpuRequest":
			cfg.Resources.CPURequest = val
		case inResources && key == "memoryRequest":
			cfg.Resources.MemoryRequest = val
		case inResources && key == "cpuLimit":
			cfg.Resources.CPULimit = val
		case inResources && key == "memoryLimit":
			cfg.Resources.MemoryLimit = val
		}
	}

	if err := scanner.Err(); err != nil {
		return cfg, fmt.Errorf("reading platform.yaml: %w", err)
	}
	return cfg, nil
}

// parseEnvItem handles two forms on one line:
//
//	name: FOO, value: bar
//	name: FOO, secretRef: my-secret/key
func parseEnvItem(item string) (EnvVar, error) {
	var ev EnvVar
	fields := strings.Split(item, ",")
	for _, f := range fields {
		k, v, ok := splitKV(strings.TrimSpace(f))
		if !ok {
			continue
		}
		switch k {
		case "name":
			ev.Name = v
		case "value":
			ev.Value = v
		case "secretRef":
			ev.SecretRef = v
		}
	}
	if ev.Name == "" {
		return ev, fmt.Errorf("env entry missing 'name'")
	}
	return ev, nil
}

func splitKV(s string) (key, val string, ok bool) {
	idx := strings.Index(s, ":")
	if idx < 0 {
		return "", "", false
	}
	key = strings.TrimSpace(s[:idx])
	val = strings.TrimSpace(s[idx+1:])
	val = strings.Trim(val, `"'`)
	return key, val, true
}

func atoiOr(s string, fallback int) int {
	n, err := strconv.Atoi(s)
	if err != nil {
		return fallback
	}
	return n
}
