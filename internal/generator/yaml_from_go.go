package generator

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"reflect"
	"sort"
	"strings"

	"github.com/iancoleman/strcase"
	"github.com/jinzhu/inflection"

	"github.com/faciam-dev/gcfm/pkg/registry"
	"github.com/faciam-dev/gcfm/pkg/registry/codec"
)

type YAMLFromGoOptions struct {
	Srcs     []string
	Merge    bool
	Existing []byte
}

func goTypeToSQL(t string) (string, bool) {
	sql, ok := GoToSQL[t]
	return sql, ok
}

func parseStruct(file string, metas *[]registry.FieldMeta) error {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, file, nil, parser.ParseComments)
	if err != nil {
		return err
	}
	for _, decl := range f.Decls {
		gd, ok := decl.(*ast.GenDecl)
		if !ok || gd.Tok != token.TYPE {
			continue
		}
		for _, spec := range gd.Specs {
			ts, ok := spec.(*ast.TypeSpec)
			if !ok {
				continue
			}
			st, ok := ts.Type.(*ast.StructType)
			if !ok {
				continue
			}
			table := strcase.ToSnake(inflection.Plural(strings.TrimSuffix(ts.Name.Name, "CF")))
			for _, fld := range st.Fields.List {
				if fld.Tag == nil || len(fld.Names) == 0 {
					continue
				}
				tag := strings.Trim(fld.Tag.Value, "`")
				if !strings.Contains(tag, "cf:\"") {
					continue
				}
				column := parseTagValue(tag, "cf")
				goType := exprString(fld.Type)
				sqlType, ok := goTypeToSQL(goType)
				if !ok {
					continue
				}
				m := registry.FieldMeta{
					TableName:  table,
					ColumnName: column,
					DataType:   sqlType,
					Validator:  parseTagValue(tag, "validate"),
				}
				*metas = append(*metas, m)
			}
		}
	}
	return nil
}

func parseTagValue(tag, key string) string {
	st := reflect.StructTag(tag)
	return st.Get(key)
}

func exprString(e ast.Expr) string {
	switch t := e.(type) {
	case *ast.StarExpr:
		return "*" + exprString(t.X)
	case *ast.SelectorExpr:
		return exprString(t.X) + "." + t.Sel.Name
	case *ast.Ident:
		return t.Name
	default:
		return ""
	}
}

func GenerateYAMLFromGo(opts YAMLFromGoOptions) ([]byte, error) {
	var metas []registry.FieldMeta
	for _, src := range opts.Srcs {
		matches, err := filepath.Glob(src)
		if err != nil {
			return nil, err
		}
		for _, m := range matches {
			if err := parseStruct(m, &metas); err != nil {
				return nil, err
			}
		}
	}
	if opts.Merge && len(opts.Existing) > 0 {
		existing, err := codec.DecodeYAML(opts.Existing)
		if err != nil {
			return nil, err
		}
		metas = mergeMetas(existing, metas)
	}
	return codec.EncodeYAML(metas)
}

func mergeMetas(old, new []registry.FieldMeta) []registry.FieldMeta {
	mp := make(map[string]registry.FieldMeta)
	for _, m := range old {
		key := m.TableName + ":" + m.ColumnName
		mp[key] = m
	}
	for _, m := range new {
		key := m.TableName + ":" + m.ColumnName
		mp[key] = m
	}
	keys := make([]string, 0, len(mp))
	for k := range mp {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var out []registry.FieldMeta
	for _, k := range keys {
		out = append(out, mp[k])
	}
	return out
}
