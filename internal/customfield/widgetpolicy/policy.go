package widgetpolicy

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync/atomic"
	"text/template"

	"github.com/fsnotify/fsnotify"
	"gopkg.in/yaml.v3"
)

// WidgetPolicy defines the configuration for auto widget resolution.
type WidgetPolicy struct {
	Version    int          `yaml:"version" json:"version"`
	SuggestTop int          `yaml:"suggest_top" json:"suggest_top"`
	Rules      []PolicyRule `yaml:"rules" json:"rules"`
}

// PolicyRule represents a single rule.
type PolicyRule struct {
	ID     string         `yaml:"id" json:"id"`
	When   RuleWhen       `yaml:"when" json:"when"`
	Widget string         `yaml:"widget" json:"widget"`
	Config map[string]any `yaml:"config" json:"config"`
	Stop   bool           `yaml:"stop" json:"stop"`
}

// RuleWhen holds conditions for matching.
type RuleWhen struct {
	Types     []string `yaml:"types" json:"types"`
	Validator []string `yaml:"validator" json:"validator"`
	Driver    []string `yaml:"driver" json:"driver"`
	LengthMin *int     `yaml:"length_min" json:"length_min"`
	LengthMax *int     `yaml:"length_max" json:"length_max"`
	NameRegex string   `yaml:"name_regex" json:"name_regex"`
}

// AutoResolveCtx is the context used when resolving widgets.
type AutoResolveCtx struct {
	Driver     string
	Type       string
	Validator  string
	Length     *int
	ColumnName string
	EnumValues []string
}

// Store watches a policy file and exposes the current policy.
type Store struct {
	path   string
	logger *slog.Logger
	val    atomic.Value // *WidgetPolicy
}

// NewStore loads the policy from path.
func NewStore(path string, logger *slog.Logger) (*Store, error) {
	s := &Store{path: path, logger: logger}
	if err := s.load(); err != nil {
		return nil, err
	}
	return s, nil
}

// Policy returns current policy.
func (s *Store) Policy() *WidgetPolicy {
	if v := s.val.Load(); v != nil {
		return v.(*WidgetPolicy)
	}
	return &WidgetPolicy{Rules: []PolicyRule{{Widget: "plugin://text-input", Stop: true}}}
}

func (s *Store) load() error {
	b, err := os.ReadFile(s.path)
	if err != nil {
		return err
	}
	pol, err := parsePolicy(b)
	if err != nil {
		return err
	}
	s.val.Store(pol)
	return nil
}

// Start watching the policy file for changes.
func (s *Store) Start(ctx context.Context) error {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}
	dir := filepath.Dir(s.path)
	if err := watcher.Add(dir); err != nil {
		watcher.Close()
		return err
	}
	go func() {
		defer watcher.Close()
		for {
			select {
			case ev := <-watcher.Events:
				if ev.Name == s.path && (ev.Op&(fsnotify.Write|fsnotify.Create)) != 0 {
					if err := s.load(); err != nil {
						s.logger.Warn("reload widget policy", "err", err)
					} else {
						s.logger.Info("widget policy reloaded")
					}
				}
			case <-ctx.Done():
				return
			case err := <-watcher.Errors:
				if err != nil {
					s.logger.Warn("widget policy watch error", "err", err)
				}
			}
		}
	}()
	return nil
}

// parsePolicy parses YAML or JSON.
func parsePolicy(b []byte) (*WidgetPolicy, error) {
	var pol WidgetPolicy
	if json.Valid(b) {
		if err := json.Unmarshal(b, &pol); err != nil {
			return nil, err
		}
		return &pol, nil
	}
	if err := yaml.Unmarshal(b, &pol); err != nil {
		return nil, err
	}
	return &pol, nil
}

// Resolve returns the first matching widget and rendered config.
func (p *WidgetPolicy) Resolve(ctx AutoResolveCtx, hasPlugin func(string) bool) (string, map[string]any) {
	for _, r := range p.Rules {
		if matchRule(r.When, ctx) {
			id := r.Widget
			cfg := renderConfig(r.Config, ctx)
			if !hasPlugin(id) && !strings.HasPrefix(id, "core://") {
				id = "plugin://text-input"
				cfg = map[string]any{}
			}
			return id, cfg
		}
	}
	return "plugin://text-input", map[string]any{}
}

// Suggest returns list of suggested widget IDs including core://auto.
func (p *WidgetPolicy) Suggest(ctx AutoResolveCtx, hasPlugin func(string) bool) []string {
	max := p.SuggestTop
	if max <= 0 {
		max = 6
	}
	suggestions := []string{"core://auto"}
	seen := map[string]struct{}{"core://auto": {}}
	for _, r := range p.Rules {
		if matchRule(r.When, ctx) {
			id := r.Widget
			if !hasPlugin(id) && !strings.HasPrefix(id, "core://") {
				id = "plugin://text-input"
			}
			if _, ok := seen[id]; !ok {
				suggestions = append(suggestions, id)
				seen[id] = struct{}{}
				if len(suggestions) >= max {
					break
				}
			}
			if r.Stop {
				break
			}
		}
	}
	if _, ok := seen["plugin://text-input"]; !ok {
		suggestions = append(suggestions, "plugin://text-input")
	}
	return suggestions
}

func matchRule(w RuleWhen, ctx AutoResolveCtx) bool {
	if !in(w.Driver, ctx.Driver) {
		return false
	}
	if !in(w.Validator, ctx.Validator) {
		return false
	}
	if !in(w.Types, ctx.Type) {
		return false
	}
	if w.LengthMin != nil {
		if ctx.Length == nil || *ctx.Length < *w.LengthMin {
			return false
		}
	}
	if w.LengthMax != nil {
		if ctx.Length != nil && *ctx.Length > *w.LengthMax {
			return false
		}
	}
	if w.NameRegex != "" {
		re := regexp.MustCompile(w.NameRegex)
		if !re.MatchString(ctx.ColumnName) {
			return false
		}
	}
	return true
}

func in(list []string, v string) bool {
	if len(list) == 0 {
		return true
	}
	v = strings.ToLower(v)
	for _, x := range list {
		if strings.ToLower(x) == v {
			return true
		}
	}
	return false
}

func renderConfig(in map[string]any, ctx AutoResolveCtx) map[string]any {
	if in == nil {
		return nil
	}
	out := make(map[string]any, len(in))
	for k, v := range in {
		switch s := v.(type) {
		case string:
			tpl, err := template.New("cfg").Funcs(template.FuncMap{
				"join": strings.Join,
				"or": func(a, b any) any {
					if isEmpty(a) {
						return b
					}
					return a
				},
				"eq": func(a, b any) bool {
					return fmt.Sprint(a) == fmt.Sprint(b)
				},
			}).Parse(s)
			if err != nil {
				out[k] = v
				continue
			}
			var buf bytes.Buffer
			_ = tpl.Execute(&buf, ctx)
			out[k] = buf.String()
		default:
			out[k] = v
		}
	}
	return out
}

func isEmpty(v any) bool {
	if v == nil {
		return true
	}
	if s, ok := v.(string); ok {
		return s == ""
	}
	return false
}

// ParseTypeInfo extracts base type, length and enum values from a column type string.
func ParseTypeInfo(t string) (base string, length *int, enums []string) {
	s := strings.ToLower(strings.TrimSpace(t))
	base = s
	if i := strings.Index(s, "("); i >= 0 && strings.HasSuffix(s, ")") {
		inner := s[i+1 : len(s)-1]
		base = s[:i]
		if base == "enum" || base == "set" {
			parts := strings.Split(inner, ",")
			for _, p := range parts {
				p = strings.TrimSpace(p)
				p = strings.Trim(p, "'\"")
				enums = append(enums, p)
			}
		} else {
			seg := strings.Split(inner, ",")[0]
			if n, err := strconv.Atoi(strings.TrimSpace(seg)); err == nil {
				length = &n
			}
		}
	}
	return
}
