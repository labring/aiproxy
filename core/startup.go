package main

import (
	"context"
	"errors"
	"fmt"
	"math/rand/v2"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"github.com/labring/aiproxy/core/common"
	"github.com/labring/aiproxy/core/common/balance"
	"github.com/labring/aiproxy/core/common/config"
	"github.com/labring/aiproxy/core/common/conv"
	"github.com/labring/aiproxy/core/common/notify"
	"github.com/labring/aiproxy/core/common/oncall"
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

	// Initialize oncall after Redis so it can use Redis for state synchronization
	oncall.Init()

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
		".env.aiproxy.local",
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

const (
	keyChars = "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
)

func generateAdminKey() string {
	key := make([]byte, 48)
	for i := range key {
		key[i] = keyChars[rand.IntN(len(keyChars))]
	}

	return conv.BytesToString(key)
}

func writeToEnvFile(envFile, key, value string) error {
	var lines []string
	if content, err := os.ReadFile(envFile); err == nil {
		lines = strings.Split(string(content), "\n")
	}

	keyPrefix := key + "="

	found := false
	for i, line := range lines {
		if strings.HasPrefix(line, keyPrefix) {
			lines[i] = key + "=" + value
			found = true
			break
		}
	}

	if !found {
		lines = append(lines, key+"="+value)
	}

	content := strings.Join(lines, "\n")
	if !strings.HasSuffix(content, "\n") && content != "" {
		content += "\n"
	}

	return os.WriteFile(envFile, []byte(content), 0o600)
}

func ensureAdminKey() error {
	if config.AdminKey != "" {
		log.Info("AdminKey is already set")
		return nil
	}

	log.Info("AdminKey is not set, generating new AdminKey...")

	config.AdminKey = generateAdminKey()

	envFile := ".env.aiproxy.local"

	absEnvFile, err := filepath.Abs(envFile)
	if err == nil {
		envFile = absEnvFile
	}

	if err := writeToEnvFile(envFile, "ADMIN_KEY", config.AdminKey); err != nil {
		return fmt.Errorf("failed to write AdminKey to %s: %w", envFile, err)
	}

	log.Info("Generated new AdminKey and saved to " + envFile)

	return nil
}
