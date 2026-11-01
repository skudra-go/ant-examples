package services

import (
	"context"
	"errors"
	"fmt"

	"github.com/skudra-go/ant"
	"github.com/skudra-go/ant/secure"

	"github.com/skudra-go/ant-examples/ledger/models"
)

// Accounts manages the chart of accounts.
type Accounts struct {
	db *models.AppDB
}

// NewAccounts returns the service.
func NewAccounts(db *models.AppDB) *Accounts { return &Accounts{db} }

// AccountInput describes an account.
type AccountInput struct {
	Code, Name, Kind, Currency string
	Parent                     string
	IBAN                       string
	Floor                      *models.Money
}

// Create adds an account.
func (s *Accounts) Create(ctx context.Context, input AccountInput) (*models.Account, error) {
	account := &models.Account{
		Code:     input.Code,
		Name:     input.Name,
		Kind:     input.Kind,
		Currency: input.Currency,
		Floor:    models.NoFloor,
		IBAN:     secure.New(input.IBAN),
	}

	if input.Floor != nil {
		account.Floor = *input.Floor
	}

	if input.Parent != "" {
		parent, err := s.byCode(ctx, s.db, input.Parent)
		if err != nil {
			return nil, err
		}

		account.ParentID = &parent.ID
	}

	err := s.db.Accounts.Insert(ctx, account)

	var invalid *ant.ValidationError

	switch {
	case errors.As(err, &invalid):
		return nil, fmt.Errorf("%w: %s %v", ErrInvalid, invalid.Field, invalid.Err)
	case ant.IsUniqueViolation(err):
		return nil, fmt.Errorf("%w: account %s exists", ErrInvalid, input.Code)
	case ant.IsCheckViolation(err):
		return nil, fmt.Errorf("%w: check the currency and floor", ErrInvalid)
	}

	return account, err
}

func (s *Accounts) byCode(
	ctx context.Context,
	db *models.AppDB,
	code string,
) (*models.Account, error) {
	account, err := db.Accounts.Where(ant.Eq(&models.Accounts.Code, code)).First(ctx)

	return account, translate(err)
}

// Node is an account and those under it, with their total.
type Node struct {
	models.Account
	Total    models.Money `json:"total"`
	Children []*Node      `json:"children,omitempty"`
}

// Tree returns the chart, with each account's balance rolled up through its
// descendants. It reads the flat list once, and builds the tree in Go.
func (s *Accounts) Tree(ctx context.Context) ([]*Node, error) {
	accounts, err := s.db.Accounts.OrderBy(&models.Accounts.Code).All(ctx)
	if err != nil {
		return nil, err
	}

	nodes := make(map[int64]*Node, len(accounts))

	for _, a := range accounts {
		nodes[a.ID] = &Node{Account: *a, Total: a.Balance}
	}

	roots := []*Node{}

	for _, a := range accounts {
		if a.ParentID == nil {
			roots = append(roots, nodes[a.ID])

			continue
		}

		if parent := nodes[*a.ParentID]; parent != nil {
			parent.Children = append(parent.Children, nodes[a.ID])
		}
	}

	var total func(*Node) models.Money

	total = func(n *Node) models.Money {
		for _, child := range n.Children {
			n.Total += total(child)
		}

		return n.Total
	}

	for _, root := range roots {
		total(root)
	}

	return roots, nil
}

// Subtree returns an account and everything under it, with one recursive query, and
// the sum of their balances.
func (s *Accounts) Subtree(
	ctx context.Context,
	code string,
) ([]*models.Account, models.Money, error) {
	accounts, err := s.db.Accounts.Where(ant.Eq(&models.Accounts.Code, code)).
		Recursive(&models.Accounts.Children).
		OrderBy(&models.Accounts.Code).
		All(ctx)
	if err != nil {
		return nil, 0, err
	}

	if len(accounts) == 0 {
		return nil, 0, ErrNotFound
	}

	var total models.Money

	for _, a := range accounts {
		total += a.Balance
	}

	return accounts, total, nil
}

// Rename changes an account's name, and takes the version the caller read. An edit
// based on what someone else already changed is refused: the version column makes the
// UPDATE match no row, which ant reports as ErrConcurrency.
func (s *Accounts) Rename(
	ctx context.Context,
	code, name string,
	version int64,
) (*models.Account, error) {
	session := ant.Session(s.db, ant.WithTracking(true))

	account, err := s.byCode(ctx, session, code)
	if err != nil {
		return nil, err
	}

	if account.Version != version {
		return nil, ErrConflict
	}

	account.Name = name

	if err := session.SaveChanges(ctx); err != nil {
		return nil, translate(err)
	}

	return account, nil
}

// Close hides an account. Its model has a DeletedAt field, so its row stays, and
// its history with it. An account holding money, or with open children, cannot close.
func (s *Accounts) Close(ctx context.Context, code string) error {
	return s.db.Tx(ctx, func(tx *models.AppDB) error {
		account, err := tx.Accounts.Where(ant.Eq(&models.Accounts.Code, code)).
			ForUpdate().
			First(ctx)
		if err != nil {
			return translate(err)
		}

		if account.Balance != 0 {
			return fmt.Errorf("%w: the account holds %s", ErrInvalid, account.Balance)
		}

		open, err := tx.Accounts.Where(ant.Eq(&models.Accounts.ParentID, &account.ID)).Exists(ctx)
		if err != nil {
			return err
		}

		if open {
			return fmt.Errorf("%w: the account has open children", ErrInvalid)
		}

		_, err = tx.Accounts.Where(ant.Eq(&models.Accounts.ID, account.ID)).Delete(ctx)

		return err
	})
}

// Reopen brings a closed account back, through the query that can see it.
func (s *Accounts) Reopen(ctx context.Context, code string) error {
	n, err := s.db.Accounts.OnlyDeleted().Where(ant.Eq(&models.Accounts.Code, code)).Restore(ctx)
	if err == nil && n == 0 {
		return ErrNotFound
	}

	return err
}

// ImportNode is an account in a chart sent as one tree.
type ImportNode struct {
	Code, Name, Kind, Currency string
	Children                   []ImportNode
}

// Import creates a whole chart from a tree in one transaction, the way TrackGraph
// is meant to be used: the structure arrives from outside, each node says what to do
// with it, and SaveChanges works out the order and fills the parent keys. An account
// that exists keeps its row, and gets the new name.
func (s *Accounts) Import(
	ctx context.Context,
	roots []ImportNode,
) (created, updated int, err error) {
	session := ant.Session(s.db, ant.WithTracking(true))

	existing, err := session.Accounts.All(ctx)
	if err != nil {
		return 0, 0, err
	}

	byCode := make(map[string]*models.Account, len(existing))

	for _, a := range existing {
		byCode[a.Code] = a
	}

	var attach func(nodes []ImportNode) []models.Account

	attach = func(nodes []ImportNode) []models.Account {
		out := make([]models.Account, 0, len(nodes))

		for _, node := range nodes {
			account := models.Account{
				Code:     node.Code,
				Name:     node.Name,
				Kind:     node.Kind,
				Currency: node.Currency,
				Floor:    models.NoFloor,
				IBAN:     secure.New(""),
			}

			if known := byCode[node.Code]; known != nil {
				account.ID = known.ID
				account.TenantID = known.TenantID
				account.Version = known.Version
				account.Balance = known.Balance
				account.CreatedAt = known.CreatedAt
				account.ParentID = known.ParentID
				account.IBAN = known.IBAN
				account.Floor = known.Floor
				account.Kind = known.Kind
				account.Currency = known.Currency
			}

			account.Children = attach(node.Children)
			out = append(out, account)
		}

		return out
	}
	top := attach(roots)

	session.ClearTracked()

	for i := range top {
		err := ant.TrackGraph(session, &top[i], func(n ant.GraphNode) ant.State {
			if n.HasKey {
				updated++

				return ant.Modified
			}

			created++

			return ant.Added
		})
		if err != nil {
			return 0, 0, err
		}
	}

	if err := session.SaveChanges(ctx); err != nil {
		var invalid *ant.ValidationError
		if errors.As(err, &invalid) {
			return 0, 0, fmt.Errorf("%w: %s %v", ErrInvalid, invalid.Field, invalid.Err)
		}

		return 0, 0, translate(err)
	}

	return created, updated, nil
}
