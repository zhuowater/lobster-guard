package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"gopkg.in/yaml.v3"
)

// ConfigPersistence 统一管理 config.yaml 的读-改-写流程，避免多个 API handler 各自实现
// 导致的持久化漂移、字段丢失和 conf.d 同步不一致。
type ConfigPersistence struct {
	mu      *sync.Mutex
	cfgPath string
}

func NewConfigPersistence(mu *sync.Mutex, cfgPath string) *ConfigPersistence {
	return &ConfigPersistence{mu: mu, cfgPath: cfgPath}
}

func (p *ConfigPersistence) LoadRaw() (map[string]interface{}, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.loadRawUnlocked()
}

func (p *ConfigPersistence) SaveRaw(raw map[string]interface{}) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.saveRawUnlocked(raw)
}

func (p *ConfigPersistence) PatchTopLevel(key string, value interface{}) error {
	return p.PatchWith(func(raw map[string]interface{}) error {
		raw[key] = normalizeYAMLValue(value)
		return nil
	})
}

func (p *ConfigPersistence) PatchSection(section string, patch map[string]interface{}) error {
	return p.PatchWith(func(raw map[string]interface{}) error {
		sub := normalizeStringMap(raw[section])
		for k, v := range patch {
			sub[k] = normalizeYAMLValue(v)
		}
		raw[section] = sub
		return nil
	})
}

func (p *ConfigPersistence) ReplaceSection(section string, value interface{}) error {
	return p.PatchWith(func(raw map[string]interface{}) error {
		raw[section] = normalizeYAMLValue(value)
		return nil
	})
}

func (p *ConfigPersistence) ReplaceSectionAndSyncConfD(section string, value interface{}) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	raw, err := p.loadRawUnlocked()
	if err != nil {
		return err
	}
	normalized := normalizeYAMLValue(value)
	raw[section] = normalized
	if err := p.saveRawUnlocked(raw); err != nil {
		return err
	}
	return p.syncConfDSectionUnlocked(section, normalized)
}

func (p *ConfigPersistence) PatchWith(fn func(raw map[string]interface{}) error) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	raw, err := p.loadRawUnlocked()
	if err != nil {
		return err
	}
	if err := fn(raw); err != nil {
		return err
	}
	return p.saveRawUnlocked(raw)
}

func (p *ConfigPersistence) SyncConfDSection(section string, value interface{}) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.syncConfDSectionUnlocked(section, normalizeYAMLValue(value))
}

func (p *ConfigPersistence) loadRawUnlocked() (map[string]interface{}, error) {
	data, err := os.ReadFile(p.cfgPath)
	if err != nil {
		return nil, fmt.Errorf("read config failed: %w", err)
	}
	var raw map[string]interface{}
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parse config failed: %w", err)
	}
	return normalizeStringMap(raw), nil
}

func (p *ConfigPersistence) saveRawUnlocked(raw map[string]interface{}) error {
	out, err := yaml.Marshal(normalizeStringMap(raw))
	if err != nil {
		return fmt.Errorf("marshal config failed: %w", err)
	}
	if err := os.WriteFile(p.cfgPath, out, 0644); err != nil {
		return fmt.Errorf("write config failed: %w", err)
	}
	return nil
}

func (p *ConfigPersistence) syncConfDSectionUnlocked(section string, value interface{}) error {
	confDir := filepath.Join(filepath.Dir(p.cfgPath), "conf.d")
	entries, err := os.ReadDir(confDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("read conf.d failed: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".yaml") {
			continue
		}
		confPath := filepath.Join(confDir, entry.Name())
		confData, err := os.ReadFile(confPath)
		if err != nil {
			return fmt.Errorf("read conf.d/%s failed: %w", entry.Name(), err)
		}
		var confRaw map[string]interface{}
		if err := yaml.Unmarshal(confData, &confRaw); err != nil {
			return fmt.Errorf("parse conf.d/%s failed: %w", entry.Name(), err)
		}
		confNorm := normalizeStringMap(confRaw)
		if _, hasSection := confNorm[section]; !hasSection {
			continue
		}
		confNorm[section] = normalizeYAMLValue(value)
		out, err := yaml.Marshal(confNorm)
		if err != nil {
			return fmt.Errorf("marshal conf.d/%s failed: %w", entry.Name(), err)
		}
		if err := os.WriteFile(confPath, out, 0644); err != nil {
			return fmt.Errorf("write conf.d/%s failed: %w", entry.Name(), err)
		}
	}
	return nil
}

func normalizeStringMap(v interface{}) map[string]interface{} {
	switch m := v.(type) {
	case nil:
		return map[string]interface{}{}
	case map[string]interface{}:
		out := make(map[string]interface{}, len(m))
		for k, val := range m {
			out[k] = normalizeYAMLValue(val)
		}
		return out
	case map[interface{}]interface{}:
		out := make(map[string]interface{}, len(m))
		for k, val := range m {
			out[fmt.Sprintf("%v", k)] = normalizeYAMLValue(val)
		}
		return out
	default:
		return map[string]interface{}{}
	}
}

func normalizeYAMLValue(v interface{}) interface{} {
	switch x := v.(type) {
	case map[string]interface{}:
		return normalizeStringMap(x)
	case map[interface{}]interface{}:
		return normalizeStringMap(x)
	case []interface{}:
		out := make([]interface{}, len(x))
		for i, item := range x {
			out[i] = normalizeYAMLValue(item)
		}
		return out
	default:
		return v
	}
}

func (api *ManagementAPI) configPersistence() *ConfigPersistence {
	return NewConfigPersistence(&api.cfgMu, api.cfgPath)
}
