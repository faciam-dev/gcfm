package plugin

import (
	"plugin"
	"sync"

	sdkplugin "github.com/faciam-dev/gcfm/sdk/plugin"
)

type Manager struct {
	mu         sync.RWMutex
	validators map[string]sdkplugin.Validator
	widgets    map[string]sdkplugin.Widget
}

func New() *Manager {
	return &Manager{
		validators: make(map[string]sdkplugin.Validator),
		widgets:    make(map[string]sdkplugin.Widget),
	}
}

func (m *Manager) Load(path string) error {
	p, err := plugin.Open(path)
	if err != nil {
		return err
	}
	if syms, err := p.Lookup("Validators"); err == nil {
		if list, ok := syms.(*[]sdkplugin.Validator); ok {
			m.mu.Lock()
			for _, v := range *list {
				m.validators[v.Name()] = v
			}
			m.mu.Unlock()
		}
	}
	if syms, err := p.Lookup("Widgets"); err == nil {
		if list, ok := syms.(*[]sdkplugin.Widget); ok {
			m.mu.Lock()
			for _, w := range *list {
				m.widgets[w.Name()] = w
			}
			m.mu.Unlock()
		}
	}
	return nil
}

func (m *Manager) Validator(name string) (sdkplugin.Validator, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	v, ok := m.validators[name]
	return v, ok
}

func (m *Manager) Widget(name string) (sdkplugin.Widget, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	w, ok := m.widgets[name]
	return w, ok
}
