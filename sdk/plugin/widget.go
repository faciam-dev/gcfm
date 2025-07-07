package plugin

type Widget interface {
	Name() string
	Schema() map[string]any
}
