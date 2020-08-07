package robots

import (
	"database/sql"
	"errors"
	"strconv"
	"time"
)

// Robot ...
type Robot struct {
	RobotID       int
	OwnerUserID   int
	ParentRobotID int
	IsFavorite    bool
	IsActive      bool
	Ticker        string
	BuyPrice      float64
	SellPrice     float64
	PlanStart     sql.NullTime
	PlanEnd       sql.NullTime
	PlanYield     float64
	FactYield     float64
	DealsCount    int
	ActivatedAt   sql.NullTime
	DeactivatedAt sql.NullTime
	CreatedAt     sql.NullTime
	DeletedAt     sql.NullTime
}

// Robots ...
type Robots interface {
	Create(r *Robot) error
	Delete(id int) error
	GetAllUserRobots(userID int) ([]*Robot, error)
	GetAllTickerRobots(ticker string) ([]*Robot, error)
	// GetAllRobots() ([]*Robot, error)
	GetRobot(id int) (*Robot, error)
	ActivateRobot(id int) error
	DeactivateRobot(id int) error
	Update(rob *Robot) error
	FavoriteRobot(rob *Robot) error
	FilterRobot(filter, how string) ([]*Robot, error)
	UpdateActual(rob *Robot) error
	GetAllNonDeletedRobots() ([]*Robot, error)
}

// FormInformationForCreate ...
func FormInformationForCreate(buy, sell, yield, planStart, planEnd string) (Robot, error) { //nolint
	var (
		rob Robot
		err error
	)

	rob.BuyPrice, err = strconv.ParseFloat(buy, 64)
	if err != nil || buy == "" {
		err = errors.New("bad buy price")

		return rob, err
	}

	rob.SellPrice, err = strconv.ParseFloat(sell, 64)
	if err != nil || sell == "" {
		err = errors.New("bad sell price")

		return Robot{}, err
	}

	rob.PlanStart.Time, err = time.Parse(time.RFC3339, planStart)
	if err != nil || planStart == "" {
		err = errors.New("bad plan start")

		return Robot{}, err
	}

	rob.PlanEnd.Time, err = time.Parse(time.RFC3339, planEnd)
	if err != nil || planEnd == "" {
		err = errors.New("bad plan end")

		return Robot{}, err
	}

	rob.PlanYield, err = strconv.ParseFloat(yield, 64)
	if err != nil || yield == "" {
		err = errors.New("bad plan yield")

		return Robot{}, err
	}

	if rob.PlanStart.Time.After(rob.PlanEnd.Time) {
		err = errors.New("plan start time shoud be erlier than end time")

		return Robot{}, err
	}

	return rob, nil
}

// ChackRobotForUpdate ...
func ChackRobotForUpdate(rob Robot) error {
	if rob.Ticker == "" {
		err := errors.New("bad ticker")

		return err
	}

	if rob.BuyPrice == 0 {
		err := errors.New("bad buy price")

		return err
	}

	if rob.SellPrice == 0 {
		err := errors.New("bad sell price")

		return err
	}

	if rob.PlanStart.Time.Equal(time.Time{}) {
		err := errors.New("bad plan start")

		return err
	}

	if rob.PlanEnd.Time.Equal(time.Time{}) {
		err := errors.New("bad plan end")

		return err
	}

	if rob.PlanYield == 0 {
		err := errors.New("bad plan yield")

		return err
	}

	if rob.PlanStart.Time.After(rob.PlanEnd.Time) {
		err := errors.New("plan start time shoud be erlier than end time")

		return err
	}

	return nil
}
