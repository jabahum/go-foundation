package rbac

type Permission struct {
	ID          string
	Name        string
	Resource    string
	Action      string
	Description string
}

type Role struct {
	ID          string
	Name        string
	Description string
}
