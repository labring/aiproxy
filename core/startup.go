package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"github.com/labring/aiproxy/core/common"
	"github.com/labring/aiproxy/core/common/balance"
	"github.com/labring/aiproxy/core/common/config"
	"github.com/labring/aiproxy/core/common/notify"
	"github.com/labring/aiproxy/core/common/pprof"
	"github.com/labring/aiproxy/core/middleware"
	"github.com/labring/aiproxy/core/model"
	"github.com/labring/aiproxy/core/router"
	log "github.com/sirupsen/logrus"
)

func initializeServices(pprofPort int) error {
	initializePprof(pprofPort)
	initializeNotifier()

	if err := common.InitRedisClient(); err != nil {
		return err
	}

	if err := initializeBalance(); err != nil {
		return err
	}

	if err := model.InitDB(); err != nil {
		return err
	}

	if err := initializeOptionAndCaches(); err != nil {
		return err
	}

	return model.InitLogDB(int(config.GetCleanLogBatchSize()))
}

func initializePprof(pprofPort int) {
	go func() {
		err := pprof.RunPprofServer(pprofPort)
		if err != nil {
			log.Errorf("run pprof server error: %v", err)
		}
	}()
}

func initializeBalance() error {
	sealosJwtKey := os.Getenv("SEALOS_JWT_KEY")
	if sealosJwtKey == "" {
		log.Info("SEALOS_JWT_KEY is not set, balance will not be enabled")
		return nil
	}

	log.Info("SEALOS_JWT_KEY is set, balance will be enabled")

	return balance.InitSealos(sealosJwtKey, os.Getenv("SEALOS_ACCOUNT_URL"))
}

func initializeNotifier() {
	feishuWh := os.Getenv("NOTIFY_FEISHU_WEBHOOK")
	if feishuWh != "" {
		notify.SetDefaultNotifier(notify.NewFeishuNotify(feishuWh))
		log.Info("NOTIFY_FEISHU_WEBHOOK is set, notifier will be use feishu")
	}
}

func initializeOptionAndCaches() error {
	log.Info("starting init config and channel")

	if err := model.InitOption2DB(); err != nil {
		return err
	}

	return model.InitModelConfigAndChannelCache()
}

func startSyncServices(ctx context.Context, wg *sync.WaitGroup) {
	wg.Add(2)

	go model.SyncOptions(ctx, wg, time.Second*5)
	go model.SyncModelConfigAndChannelCache(ctx, wg, time.Second*10)
}

func setupHTTPServer(listen string) (*http.Server, *gin.Engine) {
	server := gin.New()

	server.Use(
		middleware.GinRecoveryHandler,
		middleware.NewLog(log.StandardLogger()),
		middleware.RequestIDMiddleware,
		middleware.CORS(),
	)
	router.SetRouter(server)

	listenEnv := os.Getenv("LISTEN")
	if listenEnv != "" {
		listen = listenEnv
	}

	return &http.Server{
		Addr:              listen,
		ReadHeaderTimeout: 10 * time.Second,
		Handler:           server,
	}, server
}

var loadedEnvFiles []string

func loadEnv() {
	envfiles := []string{
		".env",
		".env.local",
	}
	for _, envfile := range envfiles {
		absPath, err := filepath.Abs(envfile)
		if err != nil {
			panic(
				fmt.Sprintf(
					"failed to get absolute path of env file: %s, error: %s",
					envfile,
					err.Error(),
				),
			)
		}

		file, err := os.Stat(absPath)
		if err != nil {
			continue
		}

		if file.IsDir() {
			continue
		}

		if err := godotenv.Overload(absPath); err != nil {
			panic(fmt.Sprintf("failed to load env file: %s, error: %s", absPath, err.Error()))
		}

		loadedEnvFiles = append(loadedEnvFiles, absPath)
	}
}

func printLoadedEnvFiles() {
	for _, envfile := range loadedEnvFiles {
		log.Infof("loaded env file: %s", envfile)
	}
}

func listenAndServe(srv *http.Server) {
	if err := srv.ListenAndServe(); err != nil &&
		!errors.Is(err, http.ErrServerClosed) {
		log.Fatal("failed to start HTTP server: " + err.Error())
	}
}
