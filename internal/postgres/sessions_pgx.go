package postgres

import (
	"authDB/internal/sessions"
	"database/sql"
	"strconv"
	"time"

	"github.com/pkg/errors"
)

var _ sessions.Sessions = &SessionStorage{}

// SessionStorage ...
type SessionStorage struct {
	statementStorage

	createStmt      *sql.Stmt
	findByIDStmt    *sql.Stmt
	findByTokenStmt *sql.Stmt
	update          *sql.Stmt
}

// NewSessionStorage ...
func NewSessionStorage(db *DB) (*SessionStorage, error) {
	s := &SessionStorage{statementStorage: newStatementsStorage(db)}

	stmts := []stmt{
		{Query: createSessionQuery, Dst: &s.createStmt},
		{Query: findSessionByIDQuery, Dst: &s.findByIDStmt},
		{Query: findSessionByTokenQuery, Dst: &s.findByTokenStmt},
		{Query: updateSessionQuery, Dst: &s.update},
	}

	if err := s.initStatements(stmts); err != nil {
		return nil, errors.Wrap(err, "can not init statements")
	}

	return s, nil
}

const sessionFields = "token, user_id, created_at, valid_until"

const createSessionQuery = "INSERT INTO public.sessions(" + sessionFields + ") VALUES ($1, $2, $3, $4)"

// Create ...
func (s *SessionStorage) Create(session *sessions.Session) error {
	_, err := s.createStmt.Exec(&session.SessionID, &session.UserID, &session.CreatedAt, &session.ValidUntil)
	if err != nil {
		err = s.Update(session.SessionID, session.UserID)
		if err != nil {
			return err
		}

		return nil
	}

	return nil
}

const findSessionByIDQuery = "SELECT " + sessionFields + " FROM public.sessions WHERE user_id=$1"

// FindByID ...
func (s *SessionStorage) FindByID(id int) (*sessions.Session, error) {
	var session sessions.Session

	idStr := strconv.Itoa(id)

	row := s.findByIDStmt.QueryRow(id)
	if err := scanSession(row, &session); err != nil {
		return nil, errors.WithMessage(err, "can not scan session"+idStr)
	}

	return &session, nil
}

const findSessionByTokenQuery = "SELECT " + sessionFields + " FROM public.sessions WHERE token=$1"

// FindByToken ...
func (s *SessionStorage) FindByToken(token string) (*sessions.Session, error) {
	var session sessions.Session

	row := s.findByTokenStmt.QueryRow(token)
	if err := scanSession(row, &session); err != nil {
		return nil, errors.WithMessage(err, "can not scan session with token"+token)
	}

	return &session, nil
}

const updateSessionQuery = "UPDATE public.sessions SET token=$1, created_at=$2, valid_until=$3 WHERE user_id=$4"

// Update ...
func (s *SessionStorage) Update(token string, id int) error {
	const min = 30

	sesDuration := min * time.Minute
	idStr := strconv.Itoa(id)

	if _, err := s.update.Exec(token, time.Now(), time.Now().Add(sesDuration), id); err != nil {
		return errors.Wrap(err, "can not exec query with userID"+idStr)
	}

	return nil
}

func scanSession(scanner sqlScanner, s *sessions.Session) error {
	return scanner.Scan(&s.SessionID, &s.UserID, &s.CreatedAt, &s.ValidUntil)
}
