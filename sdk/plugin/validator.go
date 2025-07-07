package plugin

type Validator interface {
	Name() string
	Validate(value any) error
}
