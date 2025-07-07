package main

import (
	"fmt"
	"regexp"

	sdkplugin "github.com/faciam-dev/gcfm/sdk/plugin"
)

type emailValidator struct{}

var emailRegex = regexp.MustCompile(`^[^@]+@[^@]+$`)

func (emailValidator) Name() string { return "email" }
func (emailValidator) Validate(v any) error {
	s, ok := v.(string)
	if !ok {
		return fmt.Errorf("not a string")
	}
	if !emailRegex.MatchString(s) {
		return fmt.Errorf("invalid email format")
	}
	return nil
}

type dummyWidget struct{}

func (dummyWidget) Name() string           { return "dummy" }
func (dummyWidget) Schema() map[string]any { return map[string]any{"type": "string"} }

var Validators = []sdkplugin.Validator{emailValidator{}}
var Widgets = []sdkplugin.Widget{dummyWidget{}}

func main() {}
