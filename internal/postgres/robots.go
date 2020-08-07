package postgres

import (
	"authDB/internal/robots"
	"database/sql"
	"strconv"

	"github.com/pkg/errors"
)

var _ robots.Robots = &RobotStorage{}

// RobotStorage ...
type RobotStorage struct {
	statementStorage

	createStmt             *sql.Stmt
	deleteStmt             *sql.Stmt
	getAllUserRobotsStmt   *sql.Stmt
	getAllTickerRobotsStmt *sql.Stmt
	// getAllRobotsStmt       *sql.Stmt
	getRobotStmt          *sql.Stmt
	activateRobotStmt     *sql.Stmt
	deactivateRobotStmt   *sql.Stmt
	updateRobotStmt       *sql.Stmt
	favoriteRobotStmt     *sql.Stmt
	updateActualRobotStmt *sql.Stmt
	getActualRobotStmt    *sql.Stmt
}

// NewRobotStorage ...
func NewRobotStorage(db *DB) (*RobotStorage, error) {
	s := &RobotStorage{statementStorage: newStatementsStorage(db)}

	stmts := []stmt{
		{Query: createRobotQuery, Dst: &s.createStmt},
		{Query: deleteRobotQuery, Dst: &s.deleteStmt},
		{Query: getAllUserRobotsQuery, Dst: &s.getAllUserRobotsStmt},
		{Query: getAllTickerRobotsStmtQuery, Dst: &s.getAllTickerRobotsStmt},
		// {Query: getAllRobotsStmtQuery, Dst: &s.getAllRobotsStmt},
		{Query: getRobotStmtQuery, Dst: &s.getRobotStmt},
		{Query: activateRobotStmtQuery, Dst: &s.activateRobotStmt},
		{Query: deactivateRobotStmtQuery, Dst: &s.deactivateRobotStmt},
		{Query: updateRobotQuery, Dst: &s.updateRobotStmt},
		{Query: favoriteRobotQuery, Dst: &s.favoriteRobotStmt},
		{Query: updateActualRobotStmtQuery, Dst: &s.updateActualRobotStmt},
		{Query: getAllNonDeletedRobotsStmtQuery, Dst: &s.getActualRobotStmt},
	}

	if err := s.initStatements(stmts); err != nil {
		return nil, errors.Wrap(err, "can't init statements")
	}

	return s, nil
}

const robotFields = "owner_user_id, parent_robot_id, is_favorite, is_active, ticker, buy_price, sell_price," +
	"plan_start, plan_end, plan_yield, fact_yield, deals_count, activated_at, deactivated_at, created_at, deleted_at"

const createRobotQuery = "INSERT INTO public.robots (" + robotFields + ") " +
	"VALUES ($1, 0, false, false, $2, $3, $4, $5, $6, $7, 0, 0,  null, null, now(), null)" +
	"RETURNING id;"

// Create ...
func (s *RobotStorage) Create(rob *robots.Robot) error {
	err := s.createStmt.QueryRow(rob.OwnerUserID, rob.Ticker, rob.BuyPrice, rob.SellPrice, rob.PlanStart.Time, rob.PlanEnd.Time, rob.PlanYield).Scan(&rob.RobotID)
	if err != nil {
		return errors.Wrap(err, "failed to create robot")
	}

	return nil
}

const deleteRobotQuery = "UPDATE public.robots SET deleted_at=now() WHERE id=$1"

// Delete ...
func (s *RobotStorage) Delete(id int) error {
	_, err := s.deleteStmt.Exec(id)

	if err != nil {
		idStr := strconv.Itoa(id)
		return errors.WithMessage(err, "failed to delete robot with id "+idStr)
	}

	return nil
}

const getAllUserRobotsQuery = "SELECT * FROM robots WHERE owner_user_id=$1 AND deleted_at IS NULL"

// GetAllUserRobots ...
func (s *RobotStorage) GetAllUserRobots(userID int) ([]*robots.Robot, error) {
	var rbts []*robots.Robot

	idStr := strconv.Itoa(userID)

	rows, err := s.getAllUserRobotsStmt.Query(userID)
	if err != nil {
		return nil, errors.WithMessage(err, "failed to get user robots with user_id "+idStr)
	}

	defer rows.Close()

	for rows.Next() {
		var r robots.Robot

		if err := scanRobot(rows, &r); err != nil {
			return nil, errors.WithMessage(err, "failed to scan user robots with user id"+idStr)
		}

		rbts = append(rbts, &r)
	}

	return rbts, rows.Err()
}

const getAllTickerRobotsStmtQuery = "SELECT * FROM robots WHERE ticker=$1 AND deleted_at IS NULL"

// GetAllTickerRobots ...
func (s *RobotStorage) GetAllTickerRobots(ticker string) ([]*robots.Robot, error) {
	var rbts []*robots.Robot

	rows, err := s.getAllTickerRobotsStmt.Query(ticker)
	if err != nil {
		return nil, errors.WithMessage(err, "failed to get all robots with ticker"+ticker)
	}

	defer rows.Close()

	for rows.Next() {
		var r robots.Robot

		if err := scanRobot(rows, &r); err != nil {
			return nil, errors.WithMessage(err, "failed to scan user robots with ticker"+ticker)
		}

		rbts = append(rbts, &r)
	}

	return rbts, rows.Err()
}

const getRobotStmtQuery = "SELECT * FROM robots WHERE id=$1 AND deleted_at IS NULL"

// GetRobot ...
func (s *RobotStorage) GetRobot(id int) (*robots.Robot, error) {
	var robot robots.Robot

	idStr := strconv.Itoa(id)

	row := s.getRobotStmt.QueryRow(id)
	if err := scanRobot(row, &robot); err != nil {
		return nil, errors.WithMessage(err, "can not scan robot "+idStr)
	}

	return &robot, nil
}

const activateRobotStmtQuery = "UPDATE public.robots SET is_active=true, activated_at=now() WHERE id=$1 AND (now()<plan_start OR now()>plan_end) AND is_active=false"

// ActivateRobot ...
func (s *RobotStorage) ActivateRobot(id int) error {
	idStr := strconv.Itoa(id)

	_, err := s.activateRobotStmt.Exec(id)
	if err != nil {
		return errors.WithMessage(err, "failed to activate robot with id"+idStr)
	}

	return nil
}

const deactivateRobotStmtQuery = "UPDATE public.robots SET is_active=false, deactivated_at=now() WHERE id=$1 AND (now()<plan_start OR now()>plan_end) AND is_active=true"

// DeactivateRobot ...
func (s *RobotStorage) DeactivateRobot(id int) error {
	idStr := strconv.Itoa(id)

	_, err := s.deactivateRobotStmt.Exec(id)
	if err != nil {
		return errors.WithMessage(err, "failed to deactivate robot with id"+idStr)
	}

	return nil
}

const updateRobotQuery = "UPDATE public.robots SET ticker=$1, buy_price=$2, sell_price=$3, plan_start=$4, plan_end=$5, plan_yield=$6 WHERE id=$7 AND is_active=false"

// Update ...
func (s *RobotStorage) Update(rob *robots.Robot) error {
	idStr := strconv.Itoa(rob.RobotID)

	_, err := s.updateRobotStmt.Exec(rob.Ticker, rob.BuyPrice, rob.SellPrice, rob.PlanStart.Time, rob.PlanEnd.Time, rob.PlanYield, rob.RobotID)
	if err != nil {
		return errors.WithMessage(err, "failed to update robot with id"+idStr)
	}

	return nil
}

const favoriteRobotQuery = "INSERT INTO public.robots (" + robotFields + ") " +
	"VALUES ($1, $2, true, false, $3, $4, $5, $6, $7, $8, 0, 0,  null, null, now(), null)" +
	"RETURNING id;"

// FavoriteRobot ...
func (s *RobotStorage) FavoriteRobot(rob *robots.Robot) error {
	idStr := strconv.Itoa(rob.RobotID)

	err := s.favoriteRobotStmt.QueryRow(rob.OwnerUserID, rob.ParentRobotID, rob.Ticker, rob.BuyPrice, rob.SellPrice, rob.PlanStart.Time, rob.PlanEnd.Time, rob.PlanYield).Scan(&rob.RobotID)
	if err != nil {
		return errors.WithMessage(err, "failed to make favorite robot with id"+idStr)
	}

	return nil
}

// FilterRobot ...
func (s *RobotStorage) FilterRobot(filter, how string) ([]*robots.Robot, error) {
	switch filter {
	case "ticker":
		return s.GetAllTickerRobots(how)
	case "user":
		id, err := strconv.Atoi(how)

		if err != nil {
			return nil, errors.Wrap(err, "failed to strconv param 'how' on filter")
		}

		return s.GetAllUserRobots(id)
	case "":
		return s.GetAllNonDeletedRobots()
	}

	return nil, errors.New("can't find robot by filter")
}

const updateActualRobotStmtQuery = "UPDATE public.robots SET fact_yield=$1, deals_count=$2 WHERE id=$3"

// UpdateActual ...
func (s *RobotStorage) UpdateActual(rob *robots.Robot) error {
	idStr := strconv.Itoa(rob.RobotID)

	_, err := s.updateActualRobotStmt.Exec(rob.FactYield, rob.DealsCount, rob.RobotID)
	if err != nil {
		return errors.WithMessage(err, "failed to update robot with id"+idStr)
	}

	return nil
}

const getAllNonDeletedRobotsStmtQuery = "SELECT * FROM public.robots WHERE deleted_at IS NULL"

//GetAllNonDeletedRobots ...
func (s *RobotStorage) GetAllNonDeletedRobots() ([]*robots.Robot, error) { // nolint
	var rbts []*robots.Robot

	rows, err := s.getActualRobotStmt.Query()
	if err != nil {
		return nil, errors.Wrap(err, "failed to get all non deleted robots")
	}

	defer rows.Close()

	for rows.Next() {
		var r robots.Robot
		if err := scanRobot(rows, &r); err != nil {
			return nil, errors.Wrap(err, "failed to scan non deleted robots")
		}

		rbts = append(rbts, &r)
	}

	return rbts, rows.Err()
}

func scanRobot(scanner sqlScanner, r *robots.Robot) error {
	return scanner.Scan(&r.RobotID, &r.OwnerUserID, &r.ParentRobotID, &r.IsFavorite, &r.IsActive, &r.Ticker,
		&r.BuyPrice, &r.SellPrice, &r.PlanStart, &r.PlanEnd, &r.PlanYield, &r.FactYield, &r.DealsCount,
		&r.ActivatedAt, &r.DeactivatedAt, &r.CreatedAt, &r.DeletedAt)
}
