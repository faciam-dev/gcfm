package schema

type Policy struct {
	Path   string `json:"path"`
	Method string `json:"method"`
}

type Role struct {
	ID       int64    `json:"id"`
	Name     string   `json:"name"`
	Comment  string   `json:"comment,omitempty"`
	Members  int64    `json:"members"`
	Policies []Policy `json:"policies,omitempty"`
}

type User struct {
	ID       uint64   `json:"id"`
	Username string   `json:"username"`
	Roles    []string `json:"roles,omitempty"`
}
