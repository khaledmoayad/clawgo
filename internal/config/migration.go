package config

import (
	"encoding/json"
	"os"
)

// Migration represents a config schema migration step.
type Migration struct {
	Version     int
	Description string
	Migrate     func(data map[string]interface{}) map[string]interface{}
}

// Migrations is the ordered list of config migrations.
// Each migration is applied if the config's version is below the migration's Version.
// Initially empty for v1 -- the framework is in place for future migrations.
var Migrations = []Migration{}

// MigrateConfig reads a config JSON file, applies any pending migrations,
// and writes the result back. If the file has no "configVersion" field,
// it is treated as version 0. After applying migrations, the "configVersion"
// field is updated to reflect the latest applied migration.
func MigrateConfig(configPath string) error {
	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // Nothing to migrate
		}
		return err
	}

	var configMap map[string]interface{}
	if err := json.Unmarshal(data, &configMap); err != nil {
		return err
	}

	currentVersion := GetConfigVersion(configMap)
	applied := false

	for _, m := range Migrations {
		if m.Version > currentVersion {
			configMap = m.Migrate(configMap)
			configMap["configVersion"] = float64(m.Version)
			currentVersion = m.Version
			applied = true
		}
	}

	if !applied {
		return nil // No migrations needed
	}

	result, err := json.MarshalIndent(configMap, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(configPath, result, 0644)
}

// GetConfigVersion extracts the config version from a config map.
// Returns 0 if the "configVersion" field is not present or not a number.
func GetConfigVersion(data map[string]interface{}) int {
	v, ok := data["configVersion"]
	if !ok {
		return 0
	}
	switch val := v.(type) {
	case float64:
		return int(val)
	case int:
		return val
	default:
		return 0
	}
}
