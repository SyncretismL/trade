package postgres

import (
	"database/sql"

	"github.com/pkg/errors"
)

type statementStorage struct {
	db         *DB
	statements []*sql.Stmt // for graceful shutdown
}

func newStatementsStorage(db *DB) statementStorage {
	return statementStorage{db: db}
}

type stmt struct {
	Query string
	Dst   **sql.Stmt
}

func (s *statementStorage) initStatements(statements []stmt) error {
	for i := range statements {
		statement, err := s.db.Session.Prepare(statements[i].Query)
		if err != nil {
			return errors.Wrapf(err, "can not prepare query %q", statements[i].Query)
		}

		*statements[i].Dst = statement
		s.statements = append(s.statements, statement)
	}

	return nil
}
