package main

import (
	"log"
	"os"
	"text/template"
)

func main() {
	tpl := template.Must(template.ParseFiles("internal/customfield/migrator/sql/templates/0008_rbac.up.sql.tmpl"))

	files := []struct {
		Path   string
		IDType string
	}{
		{"internal/customfield/migrator/sql/0008_rbac.up.sql", "BIGINT AUTO_INCREMENT"},
		{"internal/customfield/migrator/sql/postgres/0008_rbac.up.sql", "BIGSERIAL"},
	}

	for _, f := range files {
		out, err := os.Create(f.Path)
		if err != nil {
			log.Fatalf("create %s: %v", f.Path, err)
		}
		if err := tpl.Execute(out, map[string]string{"IDType": f.IDType}); err != nil {
			out.Close()
			log.Fatalf("write %s: %v", f.Path, err)
		}
		out.Close()
	}
}
