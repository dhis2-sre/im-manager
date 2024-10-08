package user

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/dhis2-sre/im-manager/internal/errdef"

	"github.com/dhis2-sre/im-manager/pkg/model"
	"github.com/go-mail/mail"
	"golang.org/x/crypto/scrypt"
)

//goland:noinspection GoExportedFuncWithUnexportedType
func NewService(uiUrl string, passwordTokenTtl uint, repository userRepository, dailer dailer) *service {
	return &service{uiUrl, passwordTokenTtl, repository, dailer}
}

type userRepository interface {
	create(user *model.User) error
	findByEmail(email string) (*model.User, error)
	findById(ctx context.Context, id uint) (*model.User, error)
	findOrCreate(email *model.User) (*model.User, error)
	findAll(ctx context.Context) ([]*model.User, error)
	delete(ctx context.Context, id uint) error
	update(user *model.User) (*model.User, error)
	findByEmailToken(token uuid.UUID) (*model.User, error)
	save(user *model.User) error
	resetPassword(user *model.User) error
	findByPasswordResetToken(token string) (*model.User, error)
}

type dailer interface {
	DialAndSend(m ...*mail.Message) error
}

type service struct {
	uiUrl            string
	passwordTokenTtl uint
	repository       userRepository
	dailer           dailer
}

func (s service) Save(user *model.User) error {
	return s.repository.save(user)
}

func (s service) SignUp(email string, password string) (*model.User, error) {
	hashedPassword, err := hashPassword(password)
	if err != nil {
		return nil, fmt.Errorf("password hashing failed: %s", err)
	}

	user := &model.User{
		Email:      email,
		EmailToken: uuid.New(),
		Password:   hashedPassword,
	}

	err = s.sendValidationEmail(user)
	if err != nil {
		return nil, fmt.Errorf("failed to send validation email: %s", err)
	}

	err = s.repository.create(user)
	if err != nil {
		return nil, err
	}

	return user, nil
}

func (s service) sendValidationEmail(user *model.User) error {
	m := mail.NewMessage()
	m.SetHeader("From", "DHIS2 Instance Manager <no-reply@dhis2.org>")
	m.SetHeader("To", user.Email)
	m.SetHeader("Subject", "Welcome to IM")
	link := fmt.Sprintf("%s/validate/%s", s.uiUrl, user.EmailToken)
	body := fmt.Sprintf("Hello, please click the below link to verify your email.<br/>%s", link)
	m.SetBody("text/html", body)
	return s.dailer.DialAndSend(m)
}

func hashPassword(password string) (string, error) {
	// example for making salt - https://play.golang.org/p/_Aw6WeWC42I
	salt := make([]byte, 32)
	_, err := rand.Read(salt)
	if err != nil {
		return "", err
	}

	// using recommended cost parameters from - https://godoc.org/golang.org/x/crypto/scrypt
	hash, err := scrypt.Key([]byte(password), salt, 32768, 8, 1, 32)
	if err != nil {
		return "", err
	}

	hashedPassword := fmt.Sprintf("%s.%s", hex.EncodeToString(hash), hex.EncodeToString(salt))

	return hashedPassword, nil
}

func (s service) ValidateEmail(token uuid.UUID) error {
	user, err := s.repository.findByEmailToken(token)
	if err != nil {
		return err
	}

	user.Validated = true
	return s.repository.save(user)
}

func (s service) SignIn(email string, password string) (*model.User, error) {
	unauthorizedError := "invalid email and password combination"

	user, err := s.repository.findByEmail(email)
	if err != nil {
		if errdef.IsNotFound(err) {
			return nil, errdef.NewUnauthorized(unauthorizedError)
		}
		return nil, err
	}

	match, err := comparePasswords(user.Password, password)
	if err != nil {
		return nil, fmt.Errorf("password hashing failed: %s", err)
	}

	if !match {
		return nil, errdef.NewUnauthorized(unauthorizedError)
	}

	if !user.Validated {
		return nil, errdef.NewForbidden("account not validated")
	}

	return user, nil
}

func comparePasswords(storedPassword string, suppliedPassword string) (bool, error) {
	passwordAndSalt := strings.Split(storedPassword, ".")
	if len(passwordAndSalt) != 2 {
		return false, fmt.Errorf("wrong password/salt format: %s", storedPassword)
	}

	salt, err := hex.DecodeString(passwordAndSalt[1])
	if err != nil {
		return false, fmt.Errorf("unable to verify user password")
	}

	hash, err := scrypt.Key([]byte(suppliedPassword), salt, 32768, 8, 1, 32)
	if err != nil {
		return false, err
	}

	return hex.EncodeToString(hash) == passwordAndSalt[0], nil
}

func (s service) FindAll(ctx context.Context) ([]*model.User, error) {
	return s.repository.findAll(ctx)
}

func (s service) FindById(ctx context.Context, id uint) (*model.User, error) {
	return s.repository.findById(ctx, id)
}

func (s service) FindOrCreate(email string, password string) (*model.User, error) {
	hashedPassword, err := hashPassword(password)
	if err != nil {
		return nil, fmt.Errorf("failed to hash password: %s", err)
	}

	user := &model.User{
		Email:      email,
		EmailToken: uuid.New(),
		Password:   hashedPassword,
	}

	return s.repository.findOrCreate(user)
}

func (s service) Delete(ctx context.Context, id uint) error {
	return s.repository.delete(ctx, id)
}

func (s service) Update(ctx context.Context, id uint, email, password string) (*model.User, error) {
	user, err := s.repository.findById(ctx, id)
	if err != nil {
		return nil, err
	}

	if email != "" {
		user.Email = email
	}

	if password != "" {
		var err error
		user.Password, err = hashPassword(password)
		if err != nil {
			return nil, fmt.Errorf("failed to hash password: %s", err)
		}
	}

	return s.repository.update(user)
}

func (s service) sendResetPasswordEmail(user *model.User) error {
	_, err := s.repository.findByEmail(user.Email)
	if err != nil {
		if errdef.IsNotFound(err) {
			return nil
		}
		return err
	}

	m := mail.NewMessage()
	m.SetHeader("From", "DHIS2 Instance Manager <no-reply@dhis2.org>")
	m.SetHeader("To", user.Email)
	m.SetHeader("Subject", "Reset your IM password")
	link := fmt.Sprintf("%s/reset-password/%s", s.uiUrl, user.PasswordToken.String)
	body := fmt.Sprintf("Hello, please click the link below to reset your password.<br/>%s", link)
	m.SetBody("text/html", body)
	return s.dailer.DialAndSend(m)
}

func (s service) RequestPasswordReset(email string) error {
	user, err := s.repository.findByEmail(email)

	if err != nil {
		if errdef.IsNotFound(err) {
			return nil
		}
		return err
	}

	bytes := make([]byte, 64)
	if _, err := rand.Read(bytes); err != nil {
		return err
	}
	token := base64.URLEncoding.EncodeToString(bytes)

	user.PasswordToken = sql.NullString{String: token, Valid: true}
	user.PasswordTokenTTL = uint(time.Now().Unix()) + s.passwordTokenTtl

	err = s.sendResetPasswordEmail(user)
	if err != nil {
		return err
	}

	return s.repository.save(user)
}

func (s service) ResetPassword(token string, password string) error {
	user, err := s.repository.findByPasswordResetToken(token)
	if err != nil {
		return err
	}

	tokenTtl := time.Unix(int64(user.PasswordTokenTTL), 0).UTC()
	if tokenTtl.Before(time.Now()) {
		return errdef.NewBadRequest("reset token has expired")
	}

	if password != "" {
		var err error
		user.Password, err = hashPassword(password)
		if err != nil {
			return fmt.Errorf("failed to hash password: %s", err)
		}
	}

	return s.repository.resetPassword(user)
}
