package user

import (
	"errors"
	"fmt"

	"github.com/dhis2-sre/im-manager/internal/errdef"

	"github.com/dhis2-sre/im-manager/pkg/model"
	"github.com/jackc/pgconn"
	"gorm.io/gorm"
)

func NewRepository(db *gorm.DB) *repository {
	return &repository{db}
}

type repository struct {
	db *gorm.DB
}

func (r repository) create(u *model.User) error {
	err := r.db.Create(&u).Error

	var perr *pgconn.PgError
	const uniqueKeyConstraint = "23505"
	if errors.As(err, &perr) && perr.Code == uniqueKeyConstraint {
		return errdef.NewDuplicated("user %q already exists", u.Email)
	}

	return err
}

func (r repository) findAll() ([]*model.User, error) {
	var users []*model.User

	err := r.db.
		Preload("Groups").
		Preload("AdminGroups").
		Find(&users).Error
	if err != nil {
		return nil, fmt.Errorf("failed to find all users: %v", err)
	}

	return users, nil
}

func (r repository) findByEmail(email string) (*model.User, error) {
	var u *model.User
	err := r.db.
		Preload("Groups").
		Preload("AdminGroups").
		Where("email = ?", email).
		First(&u).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, errdef.NewNotFound("failed to find user with email %q", email)
	}
	return u, err
}

func (r repository) findOrCreate(user *model.User) (*model.User, error) {
	var u *model.User
	err := r.db.Where(model.User{Email: user.Email}).Attrs(model.User{Password: user.Password}).FirstOrCreate(&u).Error
	return u, err
}

func (r repository) findById(id uint) (*model.User, error) {
	var u *model.User
	err := r.db.
		Preload("Groups").
		Preload("AdminGroups").
		First(&u, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, errdef.NewNotFound("failed to find user with id %d", id)
	}
	return u, err
}
