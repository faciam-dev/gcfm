package main

import (
	"fmt"
	"strings"

	"github.com/faciam-dev/gcfm/pkg/customfield"
)

type pluginImpl struct{}

func (pluginImpl) Name() string { return "uppercase" }

func (pluginImpl) Validate(v any) error {
	s, ok := v.(string)
	if !ok {
		return fmt.Errorf("not a string")
	}
	if s != strings.ToUpper(s) {
		return fmt.Errorf("not uppercase")
	}
	return nil
}

func New() customfield.ValidatorPlugin { return pluginImpl{} }
