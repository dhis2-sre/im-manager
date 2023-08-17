package user

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/dhis2-sre/im-manager/pkg/config"
	"github.com/google/uuid"

	"github.com/dhis2-sre/im-manager/internal/errdef"

	"github.com/dhis2-sre/im-manager/pkg/model"
	"github.com/go-mail/mail"
	"golang.org/x/crypto/scrypt"
)

//goland:noinspection GoExportedFuncWithUnexportedType
func NewService(config config.Config, repository userRepository, dailer dailer) *service {
	return &service{config, repository, dailer}
}

type userRepository interface {
	create(user *model.User) error
	findByEmail(email string) (*model.User, error)
	findById(id uint) (*model.User, error)
	findOrCreate(email *model.User) (*model.User, error)
	findAll() ([]*model.User, error)
	delete(id uint) error
	update(user *model.User) (*model.User, error)
	findByEmailToken(token uuid.UUID) (*model.User, error)
	save(user *model.User) error
}

type dailer interface {
	DialAndSend(m ...*mail.Message) error
}

type service struct {
	config     config.Config
	repository userRepository
	dailer     dailer
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
		return nil, err
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
	link := fmt.Sprintf("%s/users/validate/%s", s.config.Hostname, user.EmailToken)
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

func (s service) FindAll() ([]*model.User, error) {
	return s.repository.findAll()
}

func (s service) FindById(id uint) (*model.User, error) {
	return s.repository.findById(id)
}

func (s service) FindOrCreate(email string, password string) (*model.User, error) {
	hashedPassword, err := hashPassword(password)
	if err != nil {
		return nil, fmt.Errorf("failed to hash password: %s", err)
	}

	user := &model.User{
		Email:    email,
		Password: hashedPassword,
	}

	return s.repository.findOrCreate(user)
}

func (s service) Delete(id uint) error {
	return s.repository.delete(id)
}

func (s service) Update(id uint, email, password string) (*model.User, error) {
	user, err := s.repository.findById(id)
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
