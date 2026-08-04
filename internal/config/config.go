package config

import (
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/spf13/viper"
	"go.yaml.in/yaml/v3"

	"github.com/lucasmellof/ct/internal/highlighter"
)

const Version = 1

const (
	configDirectory = ".gct"
	configFilename  = "config.yaml"
	rulesDirectory  = "rules"
)

//go:embed defaults/config.yaml
var defaultConfig []byte

//go:embed defaults/rules/default.yaml
var defaultRules []byte

var config *Config

type Config struct {
	Version        int                `yaml:"version"`
	DefaultProfile string             `yaml:"default_profile"`
	Palette        map[string]string  `yaml:"palette"`
	Profiles       map[string]Profile `yaml:"profiles"`
}

type Profile struct {
	Extends []string `yaml:"extends"`
	Rules   []Rule   `yaml:"rules"`
}

type Rule struct {
	Description string            `yaml:"description"`
	Regex       string            `yaml:"regex"`
	Style       string            `yaml:"style"`
	Color       string            `yaml:"color"`
	Priority    int               `yaml:"priority"`
	Groups      map[string]string `yaml:"groups"`
	Exclusive   bool              `yaml:"exclusive"`
}

type ruleFile struct {
	Profiles map[string]Profile `yaml:"profiles"`
}

func LoadConfig() error {
	homeDirectory, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("find home directory: %w", err)
	}
	configDirectoryPath := filepath.Join(homeDirectory, configDirectory)
	if err := os.MkdirAll(configDirectoryPath, 0o755); err != nil {
		return fmt.Errorf("create config directory: %w", err)
	}
	configPath := configFilePath(homeDirectory)
	if err := writeDefaultFile(configPath, defaultConfig); err != nil {
		return fmt.Errorf("create config: %w", err)
	}

	rulesDirectoryPath := filepath.Join(configDirectoryPath, rulesDirectory)
	if err := os.MkdirAll(rulesDirectoryPath, 0o755); err != nil {
		return fmt.Errorf("create rules directory: %w", err)
	}

	v := viper.New()
	v.SetConfigFile(configPath)

	v.SetDefault("version", Version)
	v.SetDefault("default_profile", "default")
	v.SetDefault("palette", map[string]string{
		"amber":          "#ca9102",
		"address":        "#d787ff",
		"blue":           "#0099ff",
		"blue_gray":      "#87afaf",
		"bright_red":     "#ff3333",
		"cyan":           "#65d7fd",
		"danger":         "#ff5f5f",
		"dark_red":       "#c71800",
		"gold":           "#cab902",
		"green":          "#00ff00",
		"info":           "#5fafff",
		"interface":      "#5fffff",
		"interface_blue": "#0099ff",
		"light_blue":     "#5fafff",
		"lime":           "#28c501",
		"mint":           "#03d28d",
		"number":         "#d7d787",
		"orange":         "#ffaf00",
		"red":            "#ff0000",
		"steel_blue":     "#5698c8",
		"success":        "#5fd75f",
		"warning":        "#ffd75f",
		"yellow":         "#ffff00",
		"yellow_green":   "#79bf02",
	})

	if err := v.ReadInConfig(); err != nil {
		return fmt.Errorf("failed to read config: %w", err)
	}
	if err := v.Unmarshal(&config); err != nil {
		return fmt.Errorf("failed to unmarshal config: %w", err)
	}

	if config.Version != Version {
		return fmt.Errorf("config version mismatch: expected %d, got %d", Version, config.Version)
	}
	if config.Profiles == nil {
		config.Profiles = make(map[string]Profile)
	}

	rulePaths, err := findRuleFiles(rulesDirectoryPath)
	if err != nil {
		return err
	}
	if len(rulePaths) == 0 && len(config.Profiles) == 0 {
		if err := writeDefaultFile(filepath.Join(rulesDirectoryPath, "default.yaml"), defaultRules); err != nil {
			return fmt.Errorf("create default rules: %w", err)
		}
		rulePaths, err = findRuleFiles(rulesDirectoryPath)
		if err != nil {
			return err
		}
	} else if len(rulePaths) == 0 {
		migratedRules, err := yaml.Marshal(ruleFile{Profiles: config.Profiles})
		if err != nil {
			return fmt.Errorf("encode migrated rules: %w", err)
		}
		if err := writeDefaultFile(filepath.Join(rulesDirectoryPath, "migrated.yaml"), migratedRules); err != nil {
			return fmt.Errorf("migrate rules: %w", err)
		}
		rulePaths, err = findRuleFiles(rulesDirectoryPath)
		if err != nil {
			return err
		}
	}
	if err := loadRuleFiles(rulePaths, config); err != nil {
		return err
	}

	return nil
}

func configFilePath(homeDirectory string) string {
	return filepath.Join(homeDirectory, configDirectory, configFilename)
}

func writeDefaultFile(path string, contents []byte) error {
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if os.IsExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	defer file.Close()
	_, err = file.Write(contents)
	return err
}

func findRuleFiles(directory string) ([]string, error) {
	entries, err := os.ReadDir(directory)
	if err != nil {
		return nil, fmt.Errorf("read rules directory: %w", err)
	}

	paths := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		extension := strings.ToLower(filepath.Ext(entry.Name()))
		if extension == ".yaml" || extension == ".yml" {
			paths = append(paths, filepath.Join(directory, entry.Name()))
		}
	}
	return paths, nil
}

func loadRuleFiles(paths []string, target *Config) error {
	profileSources := make(map[string]string)
	for _, path := range paths {
		contents, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read rule file %q: %w", path, err)
		}
		var document yaml.Node
		if err := yaml.Unmarshal(contents, &document); err != nil {
			return fmt.Errorf("parse rule file %q: %w", path, err)
		}
		if len(document.Content) == 0 {
			continue
		}

		switch document.Content[0].Kind {
		case yaml.SequenceNode:
			var rules []Rule
			if err := document.Decode(&rules); err != nil {
				return fmt.Errorf("parse rule file %q: %w", path, err)
			}
			profile := target.Profiles[target.DefaultProfile]
			profile.Rules = append(profile.Rules, rules...)
			target.Profiles[target.DefaultProfile] = profile
		case yaml.MappingNode:
			var file ruleFile
			if err := document.Decode(&file); err != nil {
				return fmt.Errorf("parse rule file %q: %w", path, err)
			}
			for name, profile := range file.Profiles {
				if previousPath, exists := profileSources[name]; exists {
					return fmt.Errorf("profile %q is declared in both %q and %q", name, previousPath, path)
				}
				profileSources[name] = path
				target.Profiles[name] = profile
			}
		default:
			return fmt.Errorf("rule file %q must contain a rule list or profiles map", path)
		}
	}
	return nil
}

func HighlightRules() ([]highlighter.RuleSpec, error) {
	if config == nil {
		return nil, fmt.Errorf("config has not been loaded")
	}

	rules, err := resolveProfileRules(config.DefaultProfile, map[string]bool{})
	if err != nil {
		return nil, err
	}

	specs := make([]highlighter.RuleSpec, 0, len(rules))
	for index, rule := range rules {
		style := rule.Style
		if style == "" {
			style = rule.Color
		}
		ansi, err := styleANSI(style, config.Palette)
		if err != nil {
			return nil, fmt.Errorf("rule %q: %w", rule.Description, err)
		}
		name := rule.Description
		if name == "" {
			name = fmt.Sprintf("rule-%d", index+1)
		}
		specs = append(specs, highlighter.RuleSpec{
			Name:     name,
			Pattern:  rule.Regex,
			ANSI:     ansi,
			Priority: rule.Priority,
		})
	}
	return specs, nil
}

func resolveProfileRules(name string, visiting map[string]bool) ([]Rule, error) {
	profile, ok := config.Profiles[name]
	if !ok {
		return nil, fmt.Errorf("profile %q does not exist", name)
	}
	if visiting[name] {
		return nil, fmt.Errorf("profile inheritance cycle at %q", name)
	}
	visiting[name] = true
	defer delete(visiting, name)

	var rules []Rule
	for _, parent := range profile.Extends {
		inherited, err := resolveProfileRules(parent, visiting)
		if err != nil {
			return nil, err
		}
		rules = append(rules, inherited...)
	}
	return append(rules, profile.Rules...), nil
}

func styleANSI(style string, palette map[string]string) (string, error) {
	var codes []string
	for _, token := range strings.Fields(style) {
		switch token {
		case "bold":
			codes = append(codes, "1")
		case "dim":
			codes = append(codes, "2")
		case "italic":
			codes = append(codes, "3")
		case "underline":
			codes = append(codes, "4")
		default:
			colorName, prefix, ok := colorToken(token)
			if !ok {
				return "", fmt.Errorf("unsupported style token %q", token)
			}
			color := colorName
			var exists bool
			color, exists = palette[colorName]
			if !exists {
				return "", fmt.Errorf("palette color %q does not exist", colorName)
			}
			red, green, blue, err := parseHexColor(color)
			if err != nil {
				return "", fmt.Errorf("palette color %q: %w", colorName, err)
			}
			codes = append(codes, fmt.Sprintf("%s;2;%d;%d;%d", prefix, red, green, blue))
		}
	}
	if len(codes) == 0 {
		return "", fmt.Errorf("style is empty")
	}
	return "\x1b[" + strings.Join(codes, ";") + "m", nil
}

func colorToken(token string) (colorName, prefix string, ok bool) {
	for _, candidate := range []struct {
		prefix string
		ansi   string
	}{
		{"fg.", "38"},
		{"bg.", "48"},
		{"f.", "38"},
		{"b.", "48"},
	} {
		if value := strings.TrimPrefix(token, candidate.prefix); value != token {
			return value, candidate.ansi, true
		}
	}
	return "", "", false
}

func parseHexColor(value string) (int64, int64, int64, error) {
	if len(value) != 7 || value[0] != '#' {
		return 0, 0, 0, fmt.Errorf("expected #RRGGBB, got %q", value)
	}
	red, err := strconv.ParseInt(value[1:3], 16, 64)
	if err != nil {
		return 0, 0, 0, err
	}
	green, err := strconv.ParseInt(value[3:5], 16, 64)
	if err != nil {
		return 0, 0, 0, err
	}
	blue, err := strconv.ParseInt(value[5:7], 16, 64)
	if err != nil {
		return 0, 0, 0, err
	}
	return red, green, blue, nil
}
