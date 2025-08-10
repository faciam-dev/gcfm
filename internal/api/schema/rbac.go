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

type UserBrief struct {
	ID       int64    `json:"id"`
	Username string   `json:"username"`
	Roles    []string `json:"roles,omitempty"`
}

type UsersPage struct {
	Items   []UserBrief `json:"items"`
	Total   int64       `json:"total"`
	Page    int         `json:"page"`
	PerPage int         `json:"per_page"`
}

type ListUsersParams struct {
	Search        string `query:"search"`
	Page          int    `query:"page"`
	PerPage       int    `query:"per_page"`
	ExcludeRoleID int64  `query:"exclude_role_id"`
}
