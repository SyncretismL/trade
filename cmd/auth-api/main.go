package main

import (
	"authDB/internal/fintech"
	"authDB/internal/postgres"
	"authDB/internal/robots"
	"authDB/pkg/logger"
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"time"

	"github.com/go-chi/chi"
	"github.com/gorilla/websocket"
	"google.golang.org/grpc"
)

const (
	port  = ":8080"
	delay = 5
)

func main() { // nolint
	newLogger, err := logger.NewLogger()
	if err != nil {
		log.Fatalf("Could not instantiate log %+s", err)
	}

	db := postgres.New(newLogger)

	defer db.Close()

	repoRobot, err := postgres.NewRobotStorage(db)
	if err != nil {
		newLogger.Fatalf("failed to create robot storage %+s", err)
	}

	repoUser, err := postgres.NewUserStorage(db)
	if err != nil {
		newLogger.Fatalf("failed to create user storage %+s", err)
	}

	repoSession, err := postgres.NewSessionStorage(db)
	if err != nil {
		newLogger.Fatalf("failed to create session storage %+s", err)
	}

	conn, err := grpc.Dial("localhost:5000", grpc.WithInsecure())
	if err != nil {
		newLogger.Fatalf("can not connect to server: %+s", err)
	}

	defer conn.Close()

	var wsClients = &wsClients{
		make(map[int][]*websocket.Conn),
		make(map[int]chan *robots.Robot),
		make(map[int]*Custom),
		sync.Mutex{},
	}

	templates := ParseTemplates()
	StreamClient := fintech.NewTradingServiceClient(conn)
	handler := newHandler(newLogger, repoUser, repoSession, repoRobot, StreamClient, templates, wsClients)

	r := chi.NewRouter()

	handler.Routers(r)

	srv := &http.Server{
		Addr:    port,
		Handler: r,
	}

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)

	ctx, cancel := context.WithCancel(context.Background())

	go func() {
		oscall := <-c
		log.Printf("system call:%+v", oscall)
		cancel()
	}()

	go handler.Robot(repoRobot)

	go func() {
		err = srv.ListenAndServe()
		if err != nil {
			newLogger.Fatalf("server stopped %+s", err)
		}
	}()

	log.Printf("server started")

	<-ctx.Done()

	log.Printf("server stopped")

	ctxShutDown, cancel := context.WithTimeout(context.Background(), delay*time.Second)

	defer func() {
		cancel()
	}()

	err = srv.Shutdown(ctxShutDown)
	if err != nil {
		newLogger.Fatalf("server shutdown failed:%+s", err)
	}

	log.Printf("server exited properly")

	if err == http.ErrServerClosed {
		err = nil
	}
}
