package user

import (
	"errors"
	"fmt"

	"github.com/dhis2-sre/im-manager/internal/errdef"
	"github.com/google/uuid"

	"github.com/dhis2-sre/im-manager/pkg/model"
	"gorm.io/gorm"
)

//goland:noinspection GoExportedFuncWithUnexportedType
func NewRepository(db *gorm.DB) *repository {
	return &repository{db}
}

type repository struct {
	db *gorm.DB
}

func (r repository) save(user *model.User) error {
	return r.db.Save(&user).Error
}

func (r repository) create(u *model.User) error {
	err := r.db.Create(&u).Error
	if errors.Is(err, gorm.ErrDuplicatedKey) {
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

func (r repository) findByEmailToken(token uuid.UUID) (*model.User, error) {
	var u *model.User
	err := r.db.
		Find(&u, "email_token = ?", token).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, errdef.NewNotFound("failed to find user with email token %q", token.String())
	}
	return u, err
}

func (r repository) findOrCreate(user *model.User) (*model.User, error) {
	var u *model.User
	err := r.db.
		Where(model.User{Email: user.Email}).
		Attrs(model.User{EmailToken: user.EmailToken, Password: user.Password}).
		FirstOrCreate(&u).Error
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

func (r repository) delete(id uint) error {
	db := r.db.Unscoped().Delete(&model.User{}, id)
	if db.Error != nil {
		return fmt.Errorf("failed to delete user with id %d: %v", id, db.Error)
	} else if db.RowsAffected < 1 {
		return errdef.NewNotFound("failed to find user with id %d", id)
	}

	return nil
}

func (r repository) update(user *model.User) (*model.User, error) {
	updatedUser := model.User{
		Email:    user.Email,
		Password: user.Password,
	}

	err := r.db.Model(&user).Updates(updatedUser).Error
	if err != nil {
		return nil, fmt.Errorf("failed to update user: %v", err)
	}

	return user, nil
}
