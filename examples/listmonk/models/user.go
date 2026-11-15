package models

import (
	"time"

	"github.com/skudra-go/ant"
	"github.com/skudra-go/ant/dialect/postgres"
)

// Role is a set of permissions, for users or for one list.
type Role struct {
	ID          int32
	Type        RoleType
	ParentID    *int32
	ListID      *int32
	Permissions postgres.Array[string]
	Name        *string
	CreatedAt   time.Time
	UpdatedAt   time.Time

	Parent *Role
	List   *List
}

// Roles is the probe.
var Roles = ant.Fields[Role]()

func (Role) Configure(b *ant.ModelBuilder[Role]) {
	b.Column(&Roles.Type).Enum("role_type", "user", "list").Default("'user'::role_type")
	b.Column(&Roles.Permissions).NotNull().Default("'{}'::text[]")
	b.Column(&Roles.CreatedAt).Nullable().Default("now()")
	b.Column(&Roles.UpdatedAt).Nullable().Default("now()")

	b.BelongsTo(&Roles.Parent).
		ForeignKey(&Roles.ParentID).
		OnDelete(ant.Cascade).
		OnUpdate(ant.Cascade).
		NoIndex()

	b.BelongsTo(&Roles.List).OnDelete(ant.Cascade).OnUpdate(ant.Cascade).NoIndex()
	b.Index(&Roles.ParentID).And(&Roles.ListID).Unique().Name("idx_roles")
	b.Index(&Roles.Type).And(&Roles.Name).Unique().Where("name IS NOT NULL").Name("idx_roles_name")
}

// User is an admin user or an API user.
type User struct {
	ID            int32
	Username      string `ant:"unique"`
	PasswordLogin bool
	Password      *string `json:"-"`
	Email         string  `ant:"unique"`
	Name          string
	Avatar        *string
	Type          UserType
	UserRoleID    int32
	ListRoleID    *int32
	Status        UserStatus
	LoggedinAt    *time.Time
	CreatedAt     time.Time
	UpdatedAt     time.Time

	UserRole *Role
	ListRole *Role
}

// Users is the probe.
var Users = ant.Fields[User]()

func (User) Configure(b *ant.ModelBuilder[User]) {
	b.Column(&Users.PasswordLogin).Default("false")
	b.Column(&Users.Type).Enum("user_type", "user", "api").Default("'user'::user_type")

	b.Column(&Users.Status).
		Enum("user_status", "enabled", "disabled").
		Default("'disabled'::user_status")

	b.Column(&Users.CreatedAt).Nullable().Default("now()")
	b.Column(&Users.UpdatedAt).Nullable().Default("now()")
	b.BelongsTo(&Users.UserRole).ForeignKey(&Users.UserRoleID).OnDelete(ant.Restrict).NoIndex()
	b.BelongsTo(&Users.ListRole).ForeignKey(&Users.ListRoleID).OnDelete(ant.Cascade).NoIndex()
}

// Session is a login session, kept by the session store.
type Session struct {
	ID        string
	Data      ant.JSON[map[string]any]
	CreatedAt time.Time
}

// Sessions is the probe.
var Sessions = ant.Fields[Session]()

func (Session) Configure(b *ant.ModelBuilder[Session]) {
	b.Key(&Sessions.ID).NotGenerated()
	b.Column(&Sessions.Data).Default("'{}'::jsonb")
	b.Column(&Sessions.CreatedAt).Type("timestamp without time zone").Default("now()")
	b.Index(&Sessions.ID).And(&Sessions.CreatedAt).Name("idx_sessions")
}
