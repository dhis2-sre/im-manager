package user

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"database/sql"
	"encoding/base64"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/argon2"

	"github.com/dhis2-sre/im-manager/internal/errdef"

	"github.com/dhis2-sre/im-manager/pkg/model"
	"github.com/go-mail/mail"
)

func NewService(uiUrl string, passwordTokenTtl uint, repository *repository, dialer dailer) *Service {
	return &Service{
		uiUrl:            uiUrl,
		passwordTokenTtl: passwordTokenTtl,
		repository:       repository,
		dailer:           dialer,
	}
}

type dailer interface {
	DialAndSend(m ...*mail.Message) error
}

type Service struct {
	uiUrl            string
	passwordTokenTtl uint
	repository       *repository
	dailer           dailer
}

func (s Service) Save(ctx context.Context, user *model.User) error {
	return s.repository.save(ctx, user)
}

func (s Service) SignUp(ctx context.Context, email string, password string) (*model.User, error) {
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

	err = s.repository.create(ctx, user)
	if err != nil {
		return nil, err
	}

	return user, nil
}

func (s Service) sendValidationEmail(user *model.User) error {
	m := mail.NewMessage()
	m.SetHeader("From", "DHIS2 Instance Manager <no-reply@dhis2.org>")
	m.SetHeader("To", user.Email)
	m.SetHeader("Subject", "Welcome to IM")
	link := fmt.Sprintf("%s/validate/%s", s.uiUrl, user.EmailToken)
	body := fmt.Sprintf("Hello, please click the below link to verify your email.<br/>%s", link)
	m.SetBody("text/html", body)
	return s.dailer.DialAndSend(m)
}

const (
	argon2Iterations = 3
	argon2Memory     = 128 * 1024
	argon2Threads    = 4
	argon2KeyLen     = 32
	argon2SaltLen    = 16

	minPasswordLength = 24
)

// hashPassword returns an Argon2id-encoded hash in the standard
// $argon2id$v=19$m=...,t=...,p=...$salt$hash format.
func hashPassword(password string) (string, error) {
	if len(password) < minPasswordLength {
		return "", fmt.Errorf("password must be at least %d characters long", minPasswordLength)
	}

	salt := make([]byte, argon2SaltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("failed to generate salt: %w", err)
	}

	hash := argon2.IDKey([]byte(password), salt, argon2Iterations, argon2Memory, argon2Threads, argon2KeyLen)

	return fmt.Sprintf("$argon2id$v=19$m=%d,t=%d,p=%d$%s$%s",
		argon2Memory, argon2Iterations, argon2Threads,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(hash),
	), nil
}

func (s Service) ValidateEmail(ctx context.Context, token uuid.UUID) error {
	user, err := s.repository.findByEmailToken(ctx, token)
	if err != nil {
		return err
	}

	user.Validated = true
	return s.repository.save(ctx, user)
}

func (s Service) SignIn(ctx context.Context, email string, password string) (*model.User, error) {
	unauthorizedError := "invalid email and password combination"

	user, err := s.repository.findByEmail(ctx, email)
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

// comparePasswords verifies suppliedPassword against an Argon2id-encoded hash
// using constant-time comparison.
func comparePasswords(storedPassword string, suppliedPassword string) (bool, error) {
	parts := strings.Split(storedPassword, "$")
	if len(parts) != 6 {
		return false, fmt.Errorf("invalid password hash")
	}

	var memory, iterations uint32
	var threads uint8
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &memory, &iterations, &threads); err != nil {
		return false, fmt.Errorf("invalid password parameters")
	}

	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return false, fmt.Errorf("failed to decode salt")
	}

	expectedHash, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil {
		return false, fmt.Errorf("failed to decode hash")
	}

	computedHash := argon2.IDKey([]byte(suppliedPassword), salt, iterations, memory, threads, uint32(len(expectedHash)))

	return subtle.ConstantTimeCompare(computedHash, expectedHash) == 1, nil
}

func (s Service) FindAll(ctx context.Context) ([]*model.User, error) {
	return s.repository.findAll(ctx)
}

func (s Service) FindById(ctx context.Context, id uint) (*model.User, error) {
	return s.repository.findById(ctx, id)
}

func (s Service) FindOrCreate(ctx context.Context, email string, password string) (*model.User, error) {
	hashedPassword, err := hashPassword(password)
	if err != nil {
		return nil, fmt.Errorf("failed to hash password: %s", err)
	}

	user := &model.User{
		Email:      email,
		EmailToken: uuid.New(),
		Password:   hashedPassword,
	}

	return s.repository.findOrCreate(ctx, user)
}

func (s Service) Delete(ctx context.Context, id uint) error {
	return s.repository.delete(ctx, id)
}

func (s Service) Update(ctx context.Context, id uint, email, password string) (*model.User, error) {
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

	return s.repository.update(ctx, user)
}

func (s Service) sendResetPasswordEmail(ctx context.Context, user *model.User) error {
	_, err := s.repository.findByEmail(ctx, user.Email)
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

func (s Service) RequestPasswordReset(ctx context.Context, email string) error {
	user, err := s.repository.findByEmail(ctx, email)
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

	err = s.sendResetPasswordEmail(ctx, user)
	if err != nil {
		return err
	}

	return s.repository.save(ctx, user)
}

func (s Service) ResetPassword(ctx context.Context, token string, password string) error {
	user, err := s.repository.findByPasswordResetToken(ctx, token)
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

	return s.repository.resetPassword(ctx, user)
}
