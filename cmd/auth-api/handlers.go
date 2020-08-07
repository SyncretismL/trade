package main

import (
	"authDB/internal/fintech"
	"authDB/internal/postgres"
	"authDB/internal/robots"
	"authDB/internal/sessions"
	"authDB/internal/user"
	"authDB/pkg/logger"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"strconv"
	"sync"
	"text/template"
	"time"

	"github.com/pkg/errors"

	"github.com/go-chi/chi"
	"github.com/go-chi/chi/middleware"
	"github.com/gorilla/websocket"
)

// Handler ...
type Handler struct {
	logger      logger.Logger
	repoUser    user.Users
	repoSession sessions.Sessions
	repoRobot   robots.Robots
	streamer    fintech.TradingServiceClient
	templates   map[string]*template.Template
	wsClients   *wsClients
}

// NewHandler ...
func newHandler(newLogger logger.Logger, repoUser user.Users, repoSession sessions.Sessions,
	repoRobot robots.Robots, streamer fintech.TradingServiceClient, templates map[string]*template.Template, wsClients *wsClients) *Handler {
	return &Handler{
		logger:      newLogger,
		repoUser:    repoUser,
		repoSession: repoSession,
		repoRobot:   repoRobot,
		streamer:    streamer,
		templates:   templates,
		wsClients:   wsClients,
	}
}

//Routers маршрутизация
func (h *Handler) Routers(r *chi.Mux) *chi.Mux {
	r.Use(middleware.Recoverer)

	r.Route("/api/v1", func(r chi.Router) {
		r.Get("/signup", h.signUpHelper)
		r.Post("/signup", h.CreateUser)
		r.Get("/signin", h.signInHelper)
		r.Post("/signin", h.SignIn)
		r.Get("/robots", h.FilterRobots)
		r.HandleFunc("/robots/wsrobots", h.WSRobotsUpdate)
		r.Route("/robot", func(r chi.Router) {
			r.Get("/", h.createRobotHelper)
			r.Post("/", h.CreateRobot)
			r.Route("/{ID}", func(r chi.Router) {
				r.HandleFunc("/wsrobot", h.WSSingleRobotUpdate)
				r.Get("/", h.GetRobot)
				r.Delete("/", h.DeleteRobot)
				r.Put("/", h.UpdateRobot)
				r.Put("/favorite", h.FavoriteRobot)
				r.Put("/activate", h.ActivateRobot)
				r.Put("/deactivate", h.DeactivateRobot)
			})
		})
		r.Route("/users/{ID}", func(r chi.Router) {
			r.Get("/", h.GetUser)
			r.Put("/", h.UpdateUser)
			r.Get("/robots", h.GetUserRobots)
			r.HandleFunc("/wsuserrobot", h.WSUserRobotsUpdate)
		})
	})

	return r
}

const (
	bufferSize = 1024
	hour       = 3
	sec        = 1
	jsonType   = "application/json"
)

// ParseTemplates ...
func ParseTemplates() map[string]*template.Template {
	var templates map[string]*template.Template

	if templates == nil {
		templates = make(map[string]*template.Template)
	}

	templates["robot"] = template.Must(template.ParseFiles("./template/getrobot/index.html", "./template/getrobot/base.html"))
	templates["signin"] = template.Must(template.ParseFiles("./template/signin/index.html", "./template/signin/base.html"))
	templates["user"] = template.Must(template.ParseFiles("./template/getuser/index.html", "./template/getuser/base.html"))
	templates["signup"] = template.Must(template.ParseFiles("./template/signup/index.html", "./template/signup/base.html"))
	templates["createRobot"] = template.Must(template.ParseFiles("./template/createrobot/index.html", "./template/createrobot/base.html"))
	templates["user_robots"] = template.Must(template.ParseFiles("./template/getuserrobots/index.html", "./template/getuserrobots/base.html"))
	templates["filter_robots"] = template.Must(template.ParseFiles("./template/getrobots/index.html", "./template/getrobots/base.html"))

	return templates
}

func (h *Handler) renderTemplate(w io.Writer, name string, viewModel interface{}) {
	tmpl, ok := h.templates[name]
	if !ok {
		h.logger.Fatalf("can't find template")

		return
	}

	err := tmpl.ExecuteTemplate(w, "base", viewModel)
	if err != nil {
		h.logger.Fatalf("can not execute tamplate with template and viewmodel: %s: %s", viewModel, err)

		return
	}
}

func (h *Handler) signUpHelper(w http.ResponseWriter, r *http.Request) {
	h.renderTemplate(w, "signup", "")
}

//CreateUser создание юзера
func (h *Handler) CreateUser(w http.ResponseWriter, r *http.Request) {
	var u user.User

	u.Email = r.FormValue("email")
	u.Password = r.FormValue("pass")
	u.Birthday = r.FormValue("birthday")
	u.FirstName = r.FormValue("first_name")
	u.LastName = r.FormValue("last_name")

	err := user.CheckValidUser(&u)
	if err != nil {
		http.Error(w, fmt.Sprintln(err), http.StatusBadRequest)

		return
	}

	hashedPass, err := user.HashPass(u.Password)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		h.logger.Errorf("failed to hash pass", err)

		return
	}

	_, err = time.Parse("2006-01-02", u.Birthday)
	if err != nil {
		http.Error(w, "wrong birthday data", http.StatusBadRequest)
		return
	}

	u.Password = hashedPass
	u.CreatedAt = time.Now()
	u.UpdatedAt = time.Now()

	err = h.repoUser.Create(&u)
	if err != nil {
		h.logger.Debugf("%s", err)
		http.Error(w, "user already exist", http.StatusBadRequest)

		return
	}

	w.WriteHeader(http.StatusCreated)
}

//GetUser получение юзера
func (h *Handler) GetUser(w http.ResponseWriter, r *http.Request) {
	userID, err := strconv.Atoi(chi.URLParam(r, "ID"))
	if err != nil {
		h.logger.Debugf("bad id param %s", err)
		http.Error(w, "bad id param", http.StatusBadRequest)

		return
	}

	token := r.Header.Get("Authorization")

	ses, err := h.repoSession.FindByID(userID)
	if err != nil {
		h.logger.Debugf("%s", err)
		http.Error(w, "can not find user ses", http.StatusBadRequest)

		return
	}

	checkSes := sessions.CheckValidSes(token, ses)

	if checkSes {
		user, err := h.repoUser.Find(userID)
		if err != nil {
			h.logger.Debugf("%s", err)
			http.Error(w, "can not find user", http.StatusInternalServerError)

			return
		}

		h.renderTemplate(w, "user", user)
	} else {
		w.WriteHeader(http.StatusForbidden)
	}
}

//UpdateUser получение юзера
func (h *Handler) UpdateUser(w http.ResponseWriter, r *http.Request) { //nolint
	var u user.User

	w.Header().Add("Content-type", "application/json")

	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		h.logger.Debugf("failed to read body", err)
		w.WriteHeader(http.StatusBadRequest)

		return
	}

	err = json.Unmarshal(body, &u)
	if err != nil {
		h.logger.Debugf("failed to unmarshal json", err)
		w.WriteHeader(http.StatusBadRequest)

		return
	}

	if u.Password == "" {
		http.Error(w, "enter password", http.StatusBadRequest)
		return
	}

	u.Password, u.ID, err = user.FormInformationForUpdate(u.Password, chi.URLParam(r, "ID"))
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		h.logger.Errorf("%s", err)

		return
	}

	err = user.CheckValidUser(&u)
	if err != nil {
		http.Error(w, fmt.Sprintln(err), http.StatusBadRequest)

		return
	}

	token := r.Header.Get("Authorization")

	ses, err := h.repoSession.FindByID(u.ID)
	if err != nil {
		h.logger.Debugf("failed to find ses %s", err)
		http.Error(w, "bad id param", http.StatusBadRequest)

		return
	}

	checkSes := sessions.CheckValidSes(token, ses)

	if checkSes && u.ID == ses.UserID {
		err = h.repoUser.Update(&u)
		if err != nil {
			h.logger.Errorf("%s", err)
			http.Error(w, "something went wrong", http.StatusInternalServerError)

			return
		}

		user := user.FormForUpdate(u)

		err = json.NewEncoder(w).Encode(user)
		if err != nil {
			h.logger.Errorf("failed to encode user data", err)
			w.WriteHeader(http.StatusInternalServerError)

			return
		}
	} else {
		w.WriteHeader(http.StatusForbidden)
	}
}

func (h *Handler) signInHelper(w http.ResponseWriter, r *http.Request) {
	h.renderTemplate(w, "signin", "")
}

//SignIn авторизация
func (h *Handler) SignIn(w http.ResponseWriter, r *http.Request) {
	tempEmail := r.FormValue("email")
	tempPass := r.FormValue("pass")

	tempUser, err := h.repoUser.FindByEmail(tempEmail)
	if err != nil {
		h.logger.Debugf("failed to find user %s", err)
		http.Error(w, "can not find user", http.StatusNotFound)

		return
	}

	hp, err := user.HashPass(tempPass)
	if err != nil {
		h.logger.Errorf("failed to hash pass")
		w.WriteHeader(http.StatusInternalServerError)

		return
	}

	if tempUser.Password == hp {
		token, ses := sessions.CreateSes(tempUser)

		err = h.repoSession.Create(ses)
		if err != nil {
			h.logger.Errorf("faied to create ses", err)
			w.WriteHeader(http.StatusInternalServerError)

			return
		}

		w.Header().Add("Authorization", token)
		w.WriteHeader(http.StatusOK)
	} else {
		http.Error(w, "wrong password", http.StatusConflict)
	}
}

// FilterRobots r.Get("api/v1/robots", h.FilterRobots) ///api/v1/robots?by=ticker&by=id
func (h *Handler) FilterRobots(w http.ResponseWriter, r *http.Request) { //nolint
	var filter, val string

	token := r.Header.Get("Authorization")
	content := r.Header.Get("Content-type")

	id, err := sessions.DecodeToken(token)
	if err != nil {
		http.Error(w, "bad token", http.StatusNotFound)
		return
	}

	ses, err := h.repoSession.FindByID(id)
	if err != nil {
		http.Error(w, "failed to find session", 404)

		return
	}

	checkSes := sessions.CheckValidSes(token, ses)

	if checkSes {
		values := r.URL.Query()

		switch {
		case values.Get("ticker") != "":
			filter = "ticker"
			val = values.Get("ticker")
		case values.Get("user") != "":
			filter = "user"
			val = values.Get("user")
		default:
			filter = ""
			val = ""
		}

		type rbts struct {
			Filter string
			// Com    string
			Robots []*robots.Robot
		}

		// com := fmt.Sprintf("?filter=%s&how=%s", filter, val)

		if filter == "ticker" || filter == "user" || filter == "" {
			var data rbts

			robots, err := h.repoRobot.FilterRobot(filter, val)
			if err != nil {
				h.logger.Errorf("failed to filter robots %s", err)
				w.WriteHeader(http.StatusInternalServerError)

				return
			}

			data = rbts{
				Filter: filter,
				Robots: robots,
				// Com:    com,
			}

			if content == jsonType {
				err = JSONwriter(w, robots)
				if err != nil {
					http.Error(w, "failed to get robots", http.StatusInternalServerError)

					return
				}
			} else {
				h.renderTemplate(w, "filter_robots", data)
			}
		} else {
			http.Error(w, "bad query params", http.StatusNotFound)

			return
		}
	} else {
		w.WriteHeader(http.StatusForbidden)
	}
}

func (h *Handler) createRobotHelper(w http.ResponseWriter, r *http.Request) {
	h.renderTemplate(w, "createRobot", "")
}

// CreateRobot r.Post("/api/v1/robot", h.CreateRobot)
func (h *Handler) CreateRobot(w http.ResponseWriter, r *http.Request) {
	var err error

	rob, err := robots.FormInformationForCreate(r.FormValue("buy_price"),
		r.FormValue("sell_price"), r.FormValue("plan_yield"), r.FormValue("plan_start"), r.FormValue("plan_end"))
	if err != nil {
		http.Error(w, fmt.Sprintln(err), http.StatusBadRequest)

		return
	}

	rob.Ticker = r.FormValue("ticker")
	if rob.Ticker == "" {
		http.Error(w, "bad ticker", http.StatusBadRequest)

		return
	}

	token := r.Header.Get("Authorization")

	userID, err := sessions.DecodeToken(token)
	if err != nil {
		h.logger.Debugf("bad token %s", token)
		http.Error(w, "bad token", http.StatusBadRequest)

		return
	}

	ses, err := h.repoSession.FindByID(userID)
	if err != nil {
		h.logger.Debugf("failed to find ses %s", err)
		http.Error(w, "bad token param", http.StatusNotFound)

		return
	}

	checkSes := sessions.CheckValidSes(token, ses)

	if checkSes {
		rob.OwnerUserID = userID

		err := h.repoRobot.Create(&rob)
		if err != nil {
			h.logger.Errorf("can not create user", err)
			w.WriteHeader(http.StatusInternalServerError)

			return
		}
	} else {
		w.WriteHeader(http.StatusForbidden)
	}
}

// DeleteRobot r.Delete("/api/v1/robot/{ID}", h.DeleteRobot)
func (h *Handler) DeleteRobot(w http.ResponseWriter, r *http.Request) { //nolint
	token := r.Header.Get("Authorization")

	robotID, err := strconv.Atoi(chi.URLParam(r, "ID"))
	if err != nil {
		http.Error(w, "bad param id", http.StatusNotFound)

		return
	}

	userID, err := sessions.DecodeToken(token)
	if err != nil {
		h.logger.Debugf("bad token %s", token)
		http.Error(w, "bad token", http.StatusBadRequest)

		return
	}

	ses, err := h.repoSession.FindByID(userID)
	if err != nil {
		h.logger.Debugf("failed to find ses %s", err)
		http.Error(w, "bad token param", http.StatusNotFound)

		return
	}

	rb, err := h.repoRobot.GetRobot(robotID)
	if err != nil {
		http.Error(w, "robot not found", http.StatusNotFound)

		return
	}

	checkSes := sessions.CheckValidSes(token, ses)

	if checkSes && rb.OwnerUserID == userID {
		err = h.repoRobot.Delete(robotID)
		if err != nil {
			h.logger.Debugf("robot was not found %s", err)
			http.Error(w, "robot was not found", http.StatusNotFound)

			return
		}
	} else {
		w.WriteHeader(http.StatusForbidden)
	}
}

// FavoriteRobot r.Put("/api/v1/robot/{ID}/favorite", h.FavoriteRobot)
func (h *Handler) FavoriteRobot(w http.ResponseWriter, r *http.Request) {
	token := r.Header.Get("Authorization")

	robotID, err := strconv.Atoi(chi.URLParam(r, "ID"))
	if err != nil {
		http.Error(w, "bad param id", http.StatusBadRequest)

		return
	}

	robot, err := h.repoRobot.GetRobot(robotID)
	if err != nil {
		h.logger.Debugf("robot was not found %s", err)
		http.Error(w, "robot was not found", http.StatusBadRequest)

		return
	}

	ses, err := h.repoSession.FindByToken(token)
	if err != nil {
		h.logger.Debugf("session was not found %s", err)
		http.Error(w, "session was not found", http.StatusBadRequest)

		return
	}

	checkSes := sessions.CheckValidSes(token, ses)

	if checkSes {
		robot.ParentRobotID = robotID
		robot.OwnerUserID = ses.UserID

		err = h.repoRobot.FavoriteRobot(robot)
		if err != nil {
			h.logger.Debugf("robot was not found %s", err)
			http.Error(w, "robot was not found", http.StatusNotFound)

			return
		}
	} else {
		w.WriteHeader(http.StatusForbidden)
	}
}

// ActivateRobot r.Put("/api/v1/robot/{ID}/activate", h.ActivateRobot)
func (h *Handler) ActivateRobot(w http.ResponseWriter, r *http.Request) { //nolint
	token := r.Header.Get("Authorization")

	robotID, err := strconv.Atoi(chi.URLParam(r, "ID"))
	if err != nil {
		http.Error(w, "bad param id", http.StatusBadRequest)

		return
	}

	ses, err := h.repoSession.FindByToken(token)
	if err != nil {
		h.logger.Debugf("session was not found %s", err)
		http.Error(w, "session was not found", http.StatusBadRequest)

		return
	}

	userID, err := sessions.DecodeToken(token)
	if err != nil {
		h.logger.Debugf("bad token %s", token)
		http.Error(w, "bad token", http.StatusBadRequest)

		return
	}

	robot, err := h.repoRobot.GetRobot(robotID)
	if err != nil {
		h.logger.Debugf("robot was not found %s", err)
		http.Error(w, "robot was not found", http.StatusNotFound)

		return
	}

	checkSes := sessions.CheckValidSes(token, ses)

	if checkSes && robot.OwnerUserID == userID {
		err = h.repoRobot.ActivateRobot(robotID)
		if err != nil {
			h.logger.Debugf("%s", err)
			http.Error(w, fmt.Sprint(err), http.StatusNotFound)

			return
		}
	} else {
		w.WriteHeader(http.StatusForbidden)
	}
}

// DeactivateRobot r.Put("/api/v1/robot/{ID}/deactivate", h.ActivateRobot)
func (h *Handler) DeactivateRobot(w http.ResponseWriter, r *http.Request) { //nolint
	token := r.Header.Get("Authorization")

	robotID, err := strconv.Atoi(chi.URLParam(r, "ID"))
	if err != nil {
		http.Error(w, "bad param id", http.StatusBadRequest)

		return
	}

	ses, err := h.repoSession.FindByToken(token)
	if err != nil {
		h.logger.Debugf("session was not found %s", err)
		http.Error(w, "session was not found", http.StatusBadRequest)

		return
	}

	userID, err := sessions.DecodeToken(token)
	if err != nil {
		h.logger.Debugf("bad token %s", token)
		http.Error(w, "bad token", http.StatusBadRequest)

		return
	}

	robot, err := h.repoRobot.GetRobot(robotID)
	if err != nil {
		h.logger.Debugf("robot was not found %s", err)
		http.Error(w, "robot was not found", http.StatusNotFound)

		return
	}

	checkSes := sessions.CheckValidSes(token, ses)

	if checkSes && robot.OwnerUserID == userID {
		err = h.repoRobot.DeactivateRobot(robotID)
		if err != nil {
			h.logger.Debugf("%s", err)
			http.Error(w, fmt.Sprint(err), http.StatusNotFound)

			return
		}
	} else {
		w.WriteHeader(http.StatusForbidden)
	}
}

// GetUserRobots r.Get("users/{ID}/robots", h.GetUserRobots)
func (h *Handler) GetUserRobots(w http.ResponseWriter, r *http.Request) {
	token := r.Header.Get("Authorization")
	content := r.Header.Get("Content-type")

	userID, err := strconv.Atoi(chi.URLParam(r, "ID"))
	if err != nil {
		http.Error(w, "bad param id", http.StatusNotFound)

		return
	}

	ses, err := h.repoSession.FindByToken(token)
	if err != nil {
		h.logger.Debugf("session was not found %s", err)
		http.Error(w, "session was not found", http.StatusNotFound)

		return
	}

	checkSes := sessions.CheckValidSes(token, ses)

	if checkSes {
		type rbts struct {
			UserID int
			Robots []*robots.Robot
		}

		robots, err := h.repoRobot.GetAllUserRobots(userID)
		if err != nil {
			h.logger.Errorf("failed to get robots %s", err)
			http.Error(w, "failed to get robots", http.StatusInternalServerError)

			return
		}

		data := rbts{
			UserID: userID,
			Robots: robots,
		}

		if content == jsonType {
			err = JSONwriter(w, robots)
			if err != nil {
				http.Error(w, "failed to get robots", http.StatusInternalServerError)

				return
			}
		} else {
			h.renderTemplate(w, "user_robots", data)
		}
	} else {
		w.WriteHeader(http.StatusForbidden)
	}
}

// UpdateRobot r.Put("/api/v1/robot/{ID}", h.UpdateRobot)
func (h *Handler) UpdateRobot(w http.ResponseWriter, r *http.Request) { //nolint
	var rob robots.Robot

	w.Header().Add("Content-type", "application/json")

	token := r.Header.Get("Authorization")

	robotID, err := strconv.Atoi(chi.URLParam(r, "ID"))
	if err != nil {
		http.Error(w, "bad id param", http.StatusNotFound)

		return
	}

	ses, err := h.repoSession.FindByToken(token)
	if err != nil {
		h.logger.Debugf("session was not found %s", err)
		http.Error(w, "session was not found", http.StatusNotFound)

		return
	}

	id, err := sessions.DecodeToken(token)
	if err != nil {
		http.Error(w, "bad token", http.StatusNotFound)

		return
	}

	rb, err := h.repoRobot.GetRobot(robotID)
	if err != nil {
		http.Error(w, "user not found", http.StatusNotFound)
		return
	}

	if rb.OwnerUserID == id && sessions.CheckValidSes(token, ses) {
		body, err := ioutil.ReadAll(r.Body)
		if err != nil {
			h.logger.Debugf("failed to read body", err)
			w.WriteHeader(http.StatusBadRequest)

			return
		}

		err = json.Unmarshal(body, &rob)
		if err != nil {
			h.logger.Debugf("failed to unmarshal json", err)
			w.WriteHeader(http.StatusBadRequest)

			return
		}

		err = robots.ChackRobotForUpdate(rob)
		if err != nil {
			http.Error(w, fmt.Sprintln(err), http.StatusBadRequest)
		}

		rob.RobotID = robotID

		err = h.repoRobot.Update(&rob)
		if err != nil {
			h.logger.Debugf("robot was not found %s", err)
			http.Error(w, "robot was not found", http.StatusNotFound)

			return
		}
	} else {
		w.WriteHeader(http.StatusForbidden)
	}
}

// WSRobotsUpdate ...
func (h *Handler) WSRobotsUpdate(w http.ResponseWriter, r *http.Request) {
	var upgrader = websocket.Upgrader{
		ReadBufferSize:  bufferSize,
		WriteBufferSize: bufferSize,
	}

	upgrader.CheckOrigin = func(r *http.Request) bool { return true }

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		h.logger.Errorf("can't upgrade connection: %s", err)
		http.Error(w, "can't upgrade connection", http.StatusInternalServerError)

		return
	}

	for {
		time.Sleep(sec * time.Second)

		robots, err := h.repoRobot.GetAllNonDeletedRobots()
		if err != nil {
			h.logger.Errorf("failed to get robots %s", err)
			http.Error(w, "failed to get robots", http.StatusInternalServerError)

			return
		}

		res, err := json.Marshal(robots)
		if err != nil {
			h.logger.Errorf("can't marshal message: %+s", err)
			continue
		}

		err = conn.WriteMessage(websocket.TextMessage, res)
		if err != nil {
			h.logger.Debugf("can't broadcast message: %+s", err)
			break
		}
	}
	conn.Close()
}

// WSUserRobotsUpdate ...
func (h *Handler) WSUserRobotsUpdate(w http.ResponseWriter, r *http.Request) {
	var upgrader = websocket.Upgrader{
		ReadBufferSize:  bufferSize,
		WriteBufferSize: bufferSize,
	}

	userID, err := strconv.Atoi(chi.URLParam(r, "ID"))
	if err != nil {
		h.logger.Debugf("%s", err)
		http.Error(w, "bad id param", http.StatusNotFound)

		return
	}

	upgrader.CheckOrigin = func(r *http.Request) bool { return true }

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		h.logger.Errorf("can't upgrade connection: %s", err)
		http.Error(w, "can't upgrade connection", http.StatusInternalServerError)

		return
	}

	for {
		time.Sleep(sec * time.Second)

		robots, err := h.repoRobot.GetAllUserRobots(userID)
		if err != nil {
			h.logger.Errorf("failed to get robots %s", err)
			http.Error(w, "failed to get robots", http.StatusInternalServerError)

			return
		}

		res, err := json.Marshal(robots)
		if err != nil {
			h.logger.Errorf("can't marshal message: %+s", err)
			continue
		}

		err = conn.WriteMessage(websocket.TextMessage, res)
		if err != nil {
			h.logger.Debugf("can't broadcast message: %+s", err)
			break
		}
	}

	conn.Close()
}

// WSSingleRobotUpdate ...
func (h *Handler) WSSingleRobotUpdate(w http.ResponseWriter, r *http.Request) {
	var upgrader = websocket.Upgrader{
		ReadBufferSize:  bufferSize,
		WriteBufferSize: bufferSize,
	}

	robotID, err := strconv.Atoi(chi.URLParam(r, "ID"))
	if err != nil {
		h.logger.Debugf("%s", err)
		http.Error(w, "bad id param", http.StatusNotFound)

		return
	}

	upgrader.CheckOrigin = func(r *http.Request) bool { return true }

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		h.logger.Errorf("can't upgrade connection: %s", err)
		http.Error(w, "can't upgrade connection", http.StatusInternalServerError)

		return
	}

	h.wsClients.AddConn(conn, robotID)

	for {
		robotData := <-h.wsClients.wsRobot[robotID]

		res, err := json.Marshal(robotData)
		if err != nil {
			h.logger.Errorf("can't marshal message: %+s", err)

			continue
		}

		for key, conn := range h.wsClients.wsConn[robotID] {
			err = conn.WriteMessage(websocket.TextMessage, res)
			if err != nil {
				h.wsClients.Mutex.Lock()
				h.wsClients.wsConn[robotID] = RemoveIndex(h.wsClients.wsConn[robotID], key)
				h.wsClients.Mutex.Unlock()
				h.logger.Debugf("can't broadcast message: %+s", err)

				break
			}
		}
	}
}

// GetRobot r.Get("robot/{ID}", h.GetRobot)
func (h *Handler) GetRobot(w http.ResponseWriter, r *http.Request) {
	token := r.Header.Get("Authorization")
	content := r.Header.Get("Content-type")

	robotID, err := strconv.Atoi(chi.URLParam(r, "ID"))
	if err != nil {
		http.Error(w, "bad id param", http.StatusBadRequest)
		return
	}

	ses, err := h.repoSession.FindByToken(token)
	if err != nil {
		h.logger.Debugf("session was not found %s", err)
		http.Error(w, "session was not found", http.StatusNotFound)

		return
	}

	if sessions.CheckValidSes(token, ses) {
		robot, err := h.repoRobot.GetRobot(robotID)
		if err != nil {
			h.logger.Debugf("robot was not found %s", err)
			http.Error(w, "robot was not found", http.StatusNotFound)

			return
		}

		if !robot.DeletedAt.Valid {
			if content == jsonType {
				err = JSONwriter(w, robot)
				if err != nil {
					http.Error(w, "failed to get robot", http.StatusInternalServerError)

					return
				}
			} else {
				h.renderTemplate(w, "robot", robot)
			}
		} else {
			http.Error(w, "robot was deleted", http.StatusNotFound)
		}
	} else {
		w.WriteHeader(http.StatusForbidden)
	}
}

// WSClients ...
type wsClients struct {
	wsConn  map[int][]*websocket.Conn
	wsRobot map[int]chan *robots.Robot
	Robots  map[int]*Custom
	sync.Mutex
}

// Custom ...
type Custom struct {
	Robot     *robots.Robot
	Activated bool
}

// AddConn ...
func (ws *wsClients) AddConn(conn *websocket.Conn, robotID int) {
	ws.Mutex.Lock()
	ws.wsConn[robotID] = append(ws.wsConn[robotID], conn)
	ws.Mutex.Unlock()
}

// RemoveIndex сли коннект отвалился
func RemoveIndex(s []*websocket.Conn, index int) []*websocket.Conn {
	return append(s[:index], s[index+1:]...)
}

// JSONwriter ...
func JSONwriter(w io.Writer, data interface{}) error {
	jsonData, err := json.MarshalIndent(data, "  ", "    ")
	if err != nil {
		return errors.Wrap(err, "failed to marshal robots %s")
	}

	_, err = w.Write(jsonData)
	if err != nil {
		return errors.Wrap(err, "failed to write data %s")
	}

	return nil
}

// New ...
func (h *Handler) New(robotData *robots.Robot, repoRobot *postgres.RobotStorage) { //nolint
	go func() {
		isBuying := true
		isSelling := false

		var bouht, sold float64

		res, err := h.streamer.Price(context.Background(), &fintech.PriceRequest{Ticker: robotData.Ticker})
		if err != nil {
			h.logger.Fatalf("stream failed %s", err)
		}

		h.logger.Debugf("stream is starting with ticker:%s and robotID:%v", robotData.Ticker, robotData.RobotID)

		for {
			if !(time.Now().Add(hour*time.Hour).Before(robotData.PlanEnd.Time) && time.Now().Add(hour*time.Hour).After(robotData.PlanStart.Time)) {
				h.wsClients.Robots[robotData.RobotID].Activated = false
				h.logger.Debugf("stream has ended with ticker:%s and robotID:%v", robotData.Ticker, robotData.RobotID)

				return
			}

			data, err := res.Recv()
			if err != nil {
				if err == io.EOF {
					break
				}

				h.logger.Fatalf("can't receive from server: %v", err)
			}

			if isBuying {
				if robotData.BuyPrice >= data.BuyPrice {
					isBuying = false
					isSelling = true
					bouht = data.BuyPrice
				} else {
					continue
				}
			}

			if isSelling {
				if robotData.SellPrice <= data.SellPrice {
					isBuying = true
					isSelling = false
					sold = data.SellPrice

					robotData.DealsCount++
					robotData.FactYield += sold - bouht

					if h.wsClients.wsConn[robotData.RobotID] == nil {
						err = repoRobot.UpdateActual(robotData)
						if err != nil {
							h.logger.Fatalf("failde to update data in stream %s", err)
						}

						continue
					}

					h.wsClients.wsRobot[robotData.RobotID] <- robotData

					err = repoRobot.UpdateActual(robotData)
					if err != nil {
						h.logger.Fatalf("failed to update robots by stream %s", err)
					}
				} else {
					continue
				}
			}
		}
	}()
}

// Robot ...
func (h *Handler) Robot(repoRobot *postgres.RobotStorage) {
	for {
		actualInfoRobots, err := repoRobot.GetAllNonDeletedRobots()
		if err != nil {
			h.logger.Fatalf("failed to get robots on stream %s", err)
		}

		for _, v := range actualInfoRobots {
			if _, ok := h.wsClients.Robots[v.RobotID]; ok {
				h.wsClients.Robots[v.RobotID].Robot.ActivatedAt = v.ActivatedAt
				h.wsClients.Robots[v.RobotID].Robot.DeactivatedAt = v.DeactivatedAt
				h.wsClients.Robots[v.RobotID].Robot.PlanStart = v.PlanStart
				h.wsClients.Robots[v.RobotID].Robot.PlanEnd = v.PlanEnd
				h.wsClients.Robots[v.RobotID].Robot.BuyPrice = v.BuyPrice
				h.wsClients.Robots[v.RobotID].Robot.SellPrice = v.SellPrice
				h.wsClients.Robots[v.RobotID].Robot.Ticker = v.Ticker
				h.wsClients.Robots[v.RobotID].Robot.PlanYield = v.PlanYield

				if v.IsActive && time.Now().Add(hour*time.Hour).Before(v.PlanEnd.Time) && time.Now().Add(hour*time.Hour).After(v.PlanStart.Time) && !h.wsClients.Robots[v.RobotID].Activated {
					h.wsClients.Robots[v.RobotID].Activated = true

					h.New(v, repoRobot)
				}
			} else {
				h.wsClients.Robots[v.RobotID] = &Custom{
					Robot:     v,
					Activated: false,
				}

				h.wsClients.wsRobot[v.RobotID] = make(chan *robots.Robot)
			}
		}

		time.Sleep(hour * time.Second)
	}
}
