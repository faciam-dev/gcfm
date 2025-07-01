package codec

import (
	"gopkg.in/yaml.v3"

	"github.com/faciam-dev/gcfm/internal/customfield/registry"
)

const currentVersion = "0.3"

type registryFile struct {
	Version string        `yaml:"version"`
	Fields  []fieldMetaV3 `yaml:"fields"`
}

type fieldMetaV3 struct {
	TableName   string                `yaml:"table"`
	ColumnName  string                `yaml:"column"`
	DataType    string                `yaml:"type"`
	Placeholder string                `yaml:"placeholder,omitempty"`
	Display     *registry.DisplayMeta `yaml:"display,omitempty"`
	Validator   string                `yaml:"validator,omitempty"`
	Nullable    bool                  `yaml:"nullable,omitempty"`
	Unique      bool                  `yaml:"unique,omitempty"`
	Default     *defaultYAML          `yaml:"default,omitempty"`
}

type defaultYAML struct {
	Enabled bool   `yaml:"enabled"`
	Value   string `yaml:"value,omitempty"`
}

func (d *defaultYAML) UnmarshalYAML(value *yaml.Node) error {
	switch value.Kind {
	case yaml.ScalarNode:
		if value.Tag == "!!null" {
			d.Enabled = false
			return nil
		}
		d.Enabled = true
		d.Value = value.Value
		return nil
	case yaml.MappingNode:
		var tmp struct {
			Enabled bool   `yaml:"enabled"`
			Value   string `yaml:"value"`
		}
		if err := value.Decode(&tmp); err != nil {
			return err
		}
		d.Enabled = tmp.Enabled
		d.Value = tmp.Value
		return nil
	default:
		return nil
	}
}

func (d defaultYAML) MarshalYAML() (interface{}, error) {
	if !d.Enabled {
		return map[string]any{"enabled": false}, nil
	}
	return map[string]any{"enabled": true, "value": d.Value}, nil
}

type registryFileV1 struct {
	Version string        `yaml:"version"`
	Fields  []fieldMetaV1 `yaml:"fields"`
}

type fieldMetaV1 struct {
	TableName   string `yaml:"table"`
	ColumnName  string `yaml:"column"`
	DataType    string `yaml:"type"`
	Placeholder string `yaml:"placeholder,omitempty"`
	Validator   string `yaml:"validator,omitempty"`
}

func EncodeYAML(metas []registry.FieldMeta) ([]byte, error) {
	var out []fieldMetaV3
	for _, m := range metas {
		fm := fieldMetaV3{
			TableName:   m.TableName,
			ColumnName:  m.ColumnName,
			DataType:    m.DataType,
			Placeholder: m.Placeholder,
			Display:     m.Display,
			Validator:   m.Validator,
			Nullable:    m.Nullable,
			Unique:      m.Unique,
		}
		if m.HasDefault {
			val := ""
			if m.Default != nil {
				val = *m.Default
			}
			fm.Default = &defaultYAML{Enabled: true, Value: val}
		}
		out = append(out, fm)
	}
	rf := registryFile{Version: currentVersion, Fields: out}
	return yaml.Marshal(rf)
}

func DecodeYAML(b []byte) ([]registry.FieldMeta, error) {
	var v struct {
		Version string `yaml:"version"`
	}
	if err := yaml.Unmarshal(b, &v); err != nil {
		return nil, err
	}
	if v.Version == "" || v.Version == "0.1" {
		var rf registryFileV1
		if err := yaml.Unmarshal(b, &rf); err != nil {
			return nil, err
		}
		var metas []registry.FieldMeta
		for _, f := range rf.Fields {
			m := registry.FieldMeta{
				TableName:  f.TableName,
				ColumnName: f.ColumnName,
				DataType:   f.DataType,
				Validator:  f.Validator,
			}
			if f.Placeholder != "" {
				m.Display = &registry.DisplayMeta{PlaceholderKey: f.Placeholder, Widget: "text"}
			}
			metas = append(metas, m)
		}
		return metas, nil
	}

	if v.Version == "0.2" {
		var rf2 struct {
			Version string `yaml:"version"`
			Fields  []struct {
				TableName  string                `yaml:"table"`
				ColumnName string                `yaml:"column"`
				DataType   string                `yaml:"type"`
				Display    *registry.DisplayMeta `yaml:"display,omitempty"`
				Validator  string                `yaml:"validator,omitempty"`
				Nullable   bool                  `yaml:"nullable,omitempty"`
				Unique     bool                  `yaml:"unique,omitempty"`
				Default    string                `yaml:"default,omitempty"`
			} `yaml:"fields"`
		}
		if err := yaml.Unmarshal(b, &rf2); err != nil {
			return nil, err
		}
		var metas []registry.FieldMeta
		for _, f := range rf2.Fields {
			m := registry.FieldMeta{TableName: f.TableName, ColumnName: f.ColumnName, DataType: f.DataType, Display: f.Display, Validator: f.Validator, Nullable: f.Nullable, Unique: f.Unique}
			if f.Default != "" {
				m.HasDefault = true
				val := f.Default
				m.Default = &val
			}
			metas = append(metas, m)
		}
		return metas, nil
	}

	var rf3 registryFile
	if err := yaml.Unmarshal(b, &rf3); err != nil {
		return nil, err
	}
	var metas []registry.FieldMeta
	for _, f := range rf3.Fields {
		m := registry.FieldMeta{TableName: f.TableName, ColumnName: f.ColumnName, DataType: f.DataType, Display: f.Display, Validator: f.Validator, Nullable: f.Nullable, Unique: f.Unique}
		if f.Default != nil {
			m.HasDefault = f.Default.Enabled
			if f.Default.Enabled {
				val := f.Default.Value
				m.Default = &val
			}
		}
		if m.Display == nil {
			m.Display = &registry.DisplayMeta{Widget: "text"}
		} else if m.Display.Widget == "" {
			m.Display.Widget = "text"
		}
		metas = append(metas, m)
	}
	return metas, nil
}
