package user

import (
	"crypto/md5" // nolint
	"encoding/hex"
	"strconv"
	"time"

	"github.com/pkg/errors"
)

// Users содержит методы добавления, обновления, получения сущности юзера
type Users interface {
	Find(id int) (*User, error)
	Create(u *User) error
	Update(u *User) error
	FindByEmail(email string) (*User, error)
}

// User сущность юзера
type User struct {
	ID        int       `json:"-"`
	FirstName string    `json:"first_name"`
	LastName  string    `json:"last_name"`
	Birthday  string    `json:"birthday,omitempty"`
	Email     string    `json:"email"`
	Password  string    `json:"pass"`
	UpdatedAt time.Time `json:"-"`
	CreatedAt time.Time `json:"-"`
}

// ForUpdate ...
type ForUpdate struct {
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Birthday  string `json:"birthday,omitempty"`
	Email     string `json:"email"`
}

//HashPass ...
func HashPass(pass string) (string, error) {
	hasher := md5.New() // nolint
	_, err := hasher.Write([]byte(pass))

	if err != nil {
		return "", errors.Wrap(err, "hash failed %s")
	}

	return hex.EncodeToString(hasher.Sum(nil)), nil
}

// CheckValidUser ...
func CheckValidUser(user *User) error {
	if user.Password == "" {
		err := errors.New("enter password")

		return err
	}

	if user.Email == "" {
		err := errors.New("enter email")

		return err
	}

	if user.FirstName == "" {
		err := errors.New("enter firstname")

		return err
	}

	if user.LastName == "" {
		err := errors.New("enter lastname")

		return err
	}

	return nil
}

// FormInformationForUpdate ...
func FormInformationForUpdate(pass, id string) (string, int, error) {
	hashedPass, err := HashPass(pass)
	if err != nil {
		return "", 0, errors.Wrap(err, "failed to hash pass")
	}

	userID, err := strconv.Atoi(id)
	if err != nil {
		return "", 0, errors.Wrap(err, "bad id param ")
	}

	return hashedPass, userID, nil
}

// FormForUpdate ...
func FormForUpdate(user User) ForUpdate {
	var u ForUpdate
	u.Birthday = user.Birthday
	u.Email = user.Email
	u.FirstName = user.FirstName
	u.LastName = user.LastName

	return u
}
