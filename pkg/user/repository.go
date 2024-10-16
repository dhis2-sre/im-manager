package user

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/dhis2-sre/im-manager/internal/errdef"
	"github.com/google/uuid"

	"github.com/dhis2-sre/im-manager/pkg/model"
	"gorm.io/gorm"
)

//goland:noinspection GoExportedFuncWithUnexportedType
func NewRepository(db *gorm.DB) *repository {
	return &repository{
		db: db,
	}
}

type repository struct {
	db *gorm.DB
}

func (r repository) save(ctx context.Context, user *model.User) error {
	// only use ctx for values (logging) and not cancellation signals on cud operations for now. ctx
	// cancellation can lead to rollbacks which we should decide individually.
	ctx = context.WithoutCancel(ctx)

	return r.db.WithContext(ctx).Save(&user).Error
}

func (r repository) create(ctx context.Context, u *model.User) error {
	// only use ctx for values (logging) and not cancellation signals on cud operations for now. ctx
	// cancellation can lead to rollbacks which we should decide individually.
	ctx = context.WithoutCancel(ctx)

	err := r.db.WithContext(ctx).Create(&u).Error
	if errors.Is(err, gorm.ErrDuplicatedKey) {
		return errdef.NewDuplicated("user %q already exists", u.Email)
	}

	return err
}

func (r repository) findAll(ctx context.Context) ([]*model.User, error) {
	var users []*model.User

	err := r.db.
		WithContext(ctx).
		Preload("Groups").
		Preload("AdminGroups").
		Order("Email").
		Find(&users).Error
	if err != nil {
		return nil, fmt.Errorf("failed to find all users: %v", err)
	}

	return users, nil
}

func (r repository) findByEmail(ctx context.Context, email string) (*model.User, error) {
	var u *model.User
	err := r.db.
		WithContext(ctx).
		Preload("Groups").
		Preload("AdminGroups").
		Where("email = ?", email).
		First(&u).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, errdef.NewNotFound("failed to find user with email %q", email)
	}
	return u, err
}

func (r repository) findByEmailToken(ctx context.Context, token uuid.UUID) (*model.User, error) {
	var user *model.User
	err := r.db.WithContext(ctx).First(&user, "email_token = ?", token.String()).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, errdef.NewNotFound("failed to find user with email token %q", token.String())
	}
	return user, err
}

func (r repository) findByPasswordResetToken(ctx context.Context, token string) (*model.User, error) {
	var user *model.User
	err := r.db.WithContext(ctx).First(&user, "password_token = ?", token).Error
	return user, err
}

func (r repository) findOrCreate(ctx context.Context, user *model.User) (*model.User, error) {
	// only use ctx for values (logging) and not cancellation signals on cud operations for now. ctx
	// cancellation can lead to rollbacks which we should decide individually.
	ctx = context.WithoutCancel(ctx)

	var u *model.User
	err := r.db.
		WithContext(ctx).
		Where(model.User{Email: user.Email}).
		Attrs(model.User{EmailToken: user.EmailToken, Password: user.Password}).
		FirstOrCreate(&u).Error
	return u, err
}

func (r repository) findById(ctx context.Context, id uint) (*model.User, error) {
	var u *model.User
	err := r.db.
		WithContext(ctx).
		Preload("Groups").
		Preload("AdminGroups").
		First(&u, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, errdef.NewNotFound("failed to find user with id %d", id)
	}
	return u, err
}

func (r repository) delete(ctx context.Context, id uint) error {
	// only use ctx for values (logging) and not cancellation signals on cud operations for now. ctx
	// cancellation can lead to rollbacks which we should decide individually.
	ctx = context.WithoutCancel(ctx)

	db := r.db.WithContext(ctx).Unscoped().Delete(&model.User{}, id)
	if db.Error != nil {
		return fmt.Errorf("failed to delete user with id %d: %v", id, db.Error)
	} else if db.RowsAffected < 1 {
		return errdef.NewNotFound("failed to find user with id %d", id)
	}

	return nil
}

func (r repository) update(ctx context.Context, user *model.User) (*model.User, error) {
	// only use ctx for values (logging) and not cancellation signals on cud operations for now. ctx
	// cancellation can lead to rollbacks which we should decide individually.
	ctx = context.WithoutCancel(ctx)

	updatedUser := model.User{
		Email:    user.Email,
		Password: user.Password,
	}

	err := r.db.WithContext(ctx).Model(&user).Updates(updatedUser).Error
	if err != nil {
		return nil, fmt.Errorf("failed to update user: %v", err)
	}

	return user, nil
}

func (r repository) resetPassword(ctx context.Context, user *model.User) error {
	// only use ctx for values (logging) and not cancellation signals on cud operations for now. ctx
	// cancellation can lead to rollbacks which we should decide individually.
	ctx = context.WithoutCancel(ctx)

	updatedUser := model.User{
		Password:      user.Password,
		PasswordToken: sql.NullString{String: "", Valid: false},
	}

	err := r.db.
		WithContext(ctx).
		Model(&user).
		Select("Password", "PasswordToken").
		Updates(updatedUser).Error
	if err != nil {
		return fmt.Errorf("failed to update user password: %v", err)
	}

	return nil
}
