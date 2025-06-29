package codec

import (
	"gopkg.in/yaml.v3"

	"github.com/faciam-dev/gcfm/internal/customfield/registry"
)

const currentVersion = "0.2"

type registryFile struct {
	Version string               `yaml:"version"`
	Fields  []registry.FieldMeta `yaml:"fields"`
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
	rf := registryFile{Version: currentVersion, Fields: metas}
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

	var rf2 registryFile
	if err := yaml.Unmarshal(b, &rf2); err != nil {
		return nil, err
	}
	for i := range rf2.Fields {
		if rf2.Fields[i].Display == nil {
			rf2.Fields[i].Display = &registry.DisplayMeta{Widget: "text"}
		} else if rf2.Fields[i].Display.Widget == "" {
			rf2.Fields[i].Display.Widget = "text"
		}
	}
	return rf2.Fields, nil
}
