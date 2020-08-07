package sessions

import (
	"authDB/internal/user"
	"encoding/base64"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/pkg/errors"
)

// Session сущность сессии пользователя
type Session struct {
	SessionID  string
	UserID     int
	CreatedAt  time.Time
	ValidUntil time.Time
}

// Sessions интерфейс по работе с сессиями
type Sessions interface {
	Create(session *Session) error
	FindByID(id int) (*Session, error)
	FindByToken(token string) (*Session, error)
	Update(token string, id int) error
}

//CreateToken функция, генерирующая токен их эмайла, пароля и текущего времени в base64
func CreateToken(id int, email, pass string) string {
	timeNow := time.Now()
	timeNowStr := timeNow.String()
	idStr := strconv.Itoa(id)
	data := []byte(fmt.Sprintf("%s\n%s\n%s\n%s", idStr, email, pass, timeNowStr))
	token := base64.StdEncoding.EncodeToString(data)

	return token
}

// DecodeToken для получения id юзера
func DecodeToken(token string) (int, error) {
	decodedBytes, err := base64.StdEncoding.DecodeString(token)
	if err != nil {
		return 0, errors.Wrap(err, "can not decode token")
	}

	decodedSlice := strings.Split(string(decodedBytes), "\n")

	userID, err := strconv.Atoi(decodedSlice[0])
	if err != nil {
		return 0, errors.Wrap(err, "failed to parse decode userID: %s")
	}

	return userID, nil
}

const (
	min  = 30
	hour = 3
)

//CreateSes создают сессию
func CreateSes(user *user.User) (string, *Session) {
	var ses Session

	token := CreateToken(user.ID, user.Email, user.Password)

	ses = Session{
		SessionID:  token,
		UserID:     user.ID,
		CreatedAt:  time.Now(),
		ValidUntil: time.Now().Add(min * time.Minute),
	}

	return token, &ses
}

//CheckValidSes ...
func CheckValidSes(userToken string, s *Session) bool {
	if s.SessionID == userToken && time.Now().Add(hour*time.Hour).Before(s.ValidUntil) { //3 часа, пушто временная зона, да - это костыль
		return true
	}

	return false
}
