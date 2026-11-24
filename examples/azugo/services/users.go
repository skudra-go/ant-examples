package services

import (
	"context"
	"errors"

	"github.com/skudra-go/ant"
	"github.com/skudra-go/ant-examples/playground/models"
)

type UserService struct {
	db *models.DB
}

func NewUserService(db *models.DB) *UserService {
	return &UserService{db: db}
}

func (s *UserService) GetUsers(ctx context.Context) ([]*models.User, error) {
	return s.db.Users.
		OrderBy(&models.Users.Name).
		All(ctx)
}

func (s *UserService) GetUserByID(ctx context.Context, id string) (*models.User, error) {
	return s.db.Users.Find(ctx, id)
}

func (s *UserService) CreateUser(ctx context.Context, user *models.User) error {
	_, created, err := s.db.Users.FirstOrCreate(
		ctx,
		user,
		ant.Eq(&models.Users.Email, user.Email),
	)
	if err != nil {
		return err
	}

	if !created {
		return errors.New("user already exists")
	}

	return nil
}
