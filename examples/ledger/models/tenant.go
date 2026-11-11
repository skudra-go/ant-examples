package models

import (
	"context"
	"errors"

	"github.com/skudra-go/ant"
)

// ErrNoTenant reports a query with no tenant in its context. Failing is the point:
// code that forgets the tenant gets an error, never another customer's rows.
var ErrNoTenant = errors.New("no tenant in context")

type tenantKey struct{}

// WithTenant returns a context whose queries see only this tenant's rows.
func WithTenant(ctx context.Context, tenant int64) context.Context {
	return context.WithValue(ctx, tenantKey{}, tenant)
}

// TenantOf returns the context's tenant.
func TenantOf(ctx context.Context) (int64, error) {
	if tenant, ok := ctx.Value(tenantKey{}).(int64); ok {
		return tenant, nil
	}

	return 0, ErrNoTenant
}

// Tenants is the probe for Tenant.
var Tenants = ant.Fields[Tenant]()

// Tenant is a customer of this service.
type Tenant struct {
	ID   int64  `json:"id"`
	Name string `json:"name" ant:"unique"`
}

// Tenanted is embedded by every model that belongs to a tenant, and brings
// everything that means: the column, an index, a global filter that reaches joins,
// includes, and subqueries by itself, and the value on insert. A model cannot forget
// any of them.
type Tenanted struct {
	TenantID int64 `json:"-"`
}

// ConfigureMixin implements ant.MixinConfigurer.
func (Tenanted) ConfigureMixin(m *ant.Mixin) {
	m.Filter("tenant", func(ctx context.Context) ant.Cond {
		tenant, err := TenantOf(ctx)
		if err != nil {
			return ant.FailedCond(err)
		}

		return m.Eq("tenant_id", tenant)
	})
}

// BeforeInsert implements ant.BeforeInserter, for every model that embeds this.
func (t *Tenanted) BeforeInsert(ctx context.Context) error {
	tenant, err := TenantOf(ctx)
	if err != nil {
		return err
	}

	t.TenantID = tenant

	return nil
}
