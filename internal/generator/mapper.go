package generator

var GoToSQL = map[string]string{
	"string":     "varchar(255)",
	"*string":    "varchar(255)",
	"int":        "int",
	"*int":       "int",
	"int64":      "bigint",
	"*int64":     "bigint",
	"float64":    "double",
	"*float64":   "double",
	"time.Time":  "datetime",
	"*time.Time": "datetime",
}

var SQLToGo = map[string]string{
	"varchar":      "string",
	"text":         "string",
	"varchar(255)": "string",
	"int":          "int",
	"bigint":       "int64",
	"double":       "float64",
	"decimal":      "float64",
	"float":        "float64",
	"datetime":     "time.Time",
	"date":         "time.Time",
}
