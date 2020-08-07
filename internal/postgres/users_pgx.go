package postgres

import (
	"authDB/internal/user"
	"database/sql"
	"strconv"

	"github.com/pkg/errors"
)

var _ user.Users = &UserStorage{}

// UserStorage ...
type UserStorage struct {
	statementStorage

	createStmt      *sql.Stmt
	findStmt        *sql.Stmt
	updateStmt      *sql.Stmt
	findByEmailStmt *sql.Stmt
}

// NewUserStorage ...
func NewUserStorage(db *DB) (*UserStorage, error) {
	s := &UserStorage{statementStorage: newStatementsStorage(db)}

	stmts := []stmt{
		{Query: createUserQuery, Dst: &s.createStmt},
		{Query: findUserQuery, Dst: &s.findStmt},
		{Query: updateUserQuery, Dst: &s.updateStmt},
		{Query: findUserByEmailQuery, Dst: &s.findByEmailStmt},
	}

	if err := s.initStatements(stmts); err != nil {
		return nil, errors.Wrap(err, "can not init statements")
	}

	return s, nil
}

const userFields = "firstname, lastname, birthday, email, password, created_at, updated_at"

const createUserQuery = "INSERT INTO public.users (" + userFields + ") VALUES ($1, $2, $3, $4, $5, now(), now()) RETURNING id"

// Create ...
func (s *UserStorage) Create(u *user.User) error {
	idStr := strconv.Itoa(u.ID)

	if err := s.createStmt.QueryRow(&u.FirstName, &u.LastName, &u.Birthday, &u.Email, &u.Password).Scan(&u.ID); err != nil {
		return errors.WithMessage(err, "can not exec query with userID"+idStr)
	}

	return nil
}

const findUserQuery = "SELECT id, " + userFields + " FROM public.users WHERE id=$1"

// Find ...
func (s *UserStorage) Find(id int) (*user.User, error) {
	var u user.User

	idStr := strconv.Itoa(id)

	row := s.findStmt.QueryRow(id)
	if err := scanUser(row, &u); err != nil {
		return nil, errors.WithMessage(err, "can't scan user with id"+idStr)
	}

	return &u, nil
}

const updateUserQuery = "UPDATE public.users " +
	" SET firstname=$1, lastname=$2, birthday=$3, email=$4, password=$5, updated_at=now()" +
	"WHERE id=$6" +
	"RETURNING id"

// Update ...
func (s *UserStorage) Update(u *user.User) error {
	idStr := strconv.Itoa(u.ID)

	if err := s.updateStmt.QueryRow(u.FirstName, u.LastName, u.Birthday, u.Email, u.Password, u.ID).Scan(&u.ID); err != nil {
		return errors.WithMessage(err, "can not update user with id"+idStr)
	}

	return nil
}

const findUserByEmailQuery = "SELECT id, " + userFields + " FROM public.users WHERE email=$1"

// FindByEmail ...
func (s *UserStorage) FindByEmail(email string) (*user.User, error) {
	var u user.User

	row := s.findByEmailStmt.QueryRow(email)
	if err := scanUser(row, &u); err != nil {
		return nil, errors.Wrap(err, "can't scan user with email"+email)
	}

	return &u, nil
}

func scanUser(scanner sqlScanner, u *user.User) error {
	return scanner.Scan(&u.ID, &u.FirstName, &u.LastName, &u.Birthday, &u.Email, &u.Password, &u.CreatedAt, &u.UpdatedAt)
}
