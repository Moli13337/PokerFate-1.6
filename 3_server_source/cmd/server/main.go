package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"go.uber.org/zap"

	"poker-fate-server/internal/config"
	"poker-fate-server/internal/db"
	"poker-fate-server/internal/game"
	"poker-fate-server/internal/gamedata"
	"poker-fate-server/internal/httpapi"
	"poker-fate-server/internal/ws"
)

func main() {
	logger, _ := zap.NewProduction()
	defer logger.Sync()

	cfg, err := config.Load("configs/config.yaml")
	if err != nil {
		logger.Fatal("load config failed", zap.Error(err))
	}

	pgDB, err := db.NewPostgres(cfg.Database.Postgres)
	if err != nil {
		logger.Fatal("connect postgres failed", zap.Error(err))
	}
	defer pgDB.Close()

	rdb, err := db.NewRedis(cfg.Database.Redis)
	if err != nil {
		logger.Fatal("connect redis failed", zap.Error(err))
	}
	defer rdb.Close()

	gamedata.MustLoad()
	logger.Info("gamedata loaded", zap.Int("tables", len(gamedata.TableNames())))

	if err := db.EnsureSchema(pgDB); err != nil {
		logger.Fatal("ensure schema failed", zap.Error(err))
	}

	wsSrv := ws.NewServer(cfg, pgDB, rdb, logger)
	wsSrv.RegisterAllHandlers()

	gameMgr := game.NewManager(pgDB, rdb, wsSrv, logger)
	_ = gameMgr

	router := httpapi.NewRouter(cfg, pgDB, rdb, wsSrv, logger)
	router.Setup()

	go func() {
		logger.Info("http server starting", zap.String("addr", cfg.Server.HTTPAddr))
		// Wrap the gin engine so that double-slash paths (produced when
		// http_host has a trailing slash AND the Lua path has a leading
		// slash, e.g. "http://127.0.0.1:8888//draw/list") are normalised
		// BEFORE gin's router tree sees them.
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if p := r.URL.Path; len(p) > 1 && p[0] == '/' && p[1] == '/' {
				r.URL.Path = "/" + strings.TrimLeft(p, "/")
			}
			router.Engine.ServeHTTP(w, r)
		})
		if err := http.ListenAndServe(cfg.Server.HTTPAddr, handler); err != nil && err != http.ErrServerClosed {
			logger.Fatal("http server failed", zap.Error(err))
		}
	}()

	go func() {
		if err := wsSrv.Start(cfg.Server.WSAddr); err != nil {
			logger.Fatal("ws server failed", zap.Error(err))
		}
	}()

	logger.Info(fmt.Sprintf("poker-fate server started: HTTP=%s WS=%s", cfg.Server.HTTPAddr, cfg.Server.WSAddr))

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("shutting down...")
	ctx, cancel := context.WithTimeout(context.Background(), 5)
	defer cancel()
	_ = ctx

	logger.Info("server stopped")
}
