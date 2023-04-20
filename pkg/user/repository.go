package user

import (
	"errors"
	"fmt"
	"log"

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
		err := fmt.Errorf("user %q already exists", u.Email)
		return errdef.NewDuplicated(err)
	}

	return err
}

func (r repository) findByEmail(email string) (*model.User, error) {
	log.Println("email")
	log.Println(email)
	var u *model.User
	err := r.db.
		Preload("Groups").
		Preload("AdminGroups").
		Where("email = ?", email).
		First(&u).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		err := fmt.Errorf("failed to find user with email %q", email)
		return u, errdef.NewNotFound(err)
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
		err := fmt.Errorf("failed to find user with id %d", id)
		return u, errdef.NewNotFound(err)
	}
	return u, err
}
