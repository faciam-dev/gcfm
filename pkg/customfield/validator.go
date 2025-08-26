package customfield

import (
	"errors"
	"fmt"
	"sync"
)

// ValidatorFunc validates a value. It should return an error if the value is invalid.
type ValidatorFunc func(v any) error

// ValidatorPlugin is implemented by runtime validator plugins.
type ValidatorPlugin interface {
	Name() string
	Validate(v any) error
}

var (
	mu         sync.RWMutex
	validators = make(map[string]ValidatorFunc)
	// ErrValidatorExists is returned by RegisterValidator when a
	// validator with the same name has already been registered.
	ErrValidatorExists = errors.New("validator already registered")
)

// RegisterValidator registers a validator under the given name.
// It returns an error if the name is already registered.
func RegisterValidator(name string, fn ValidatorFunc) error {
	mu.Lock()
	defer mu.Unlock()
	if _, ok := validators[name]; ok {
		return fmt.Errorf("%w: %s", ErrValidatorExists, name)
	}
	validators[name] = fn
	return nil
}

// GetValidator retrieves a validator by name.
func GetValidator(name string) (ValidatorFunc, bool) {
	mu.RLock()
	defer mu.RUnlock()
	fn, ok := validators[name]
	return fn, ok
}

// Registered returns the names of all registered validators.
func Registered() []string {
	mu.RLock()
	defer mu.RUnlock()
	names := make([]string, 0, len(validators))
	for n := range validators {
		names = append(names, n)
	}
	return names
}
