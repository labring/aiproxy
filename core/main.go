package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	stdlog "log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"slices"
	"sync"
	"syscall"
	"time"

	"github.com/bytedance/sonic"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"github.com/labring/aiproxy/core/common"
	"github.com/labring/aiproxy/core/common/balance"
	"github.com/labring/aiproxy/core/common/config"
	"github.com/labring/aiproxy/core/common/consume"
	"github.com/labring/aiproxy/core/common/conv"
	"github.com/labring/aiproxy/core/common/ipblack"
	"github.com/labring/aiproxy/core/common/notify"
	"github.com/labring/aiproxy/core/common/trylock"
	"github.com/labring/aiproxy/core/controller"
	"github.com/labring/aiproxy/core/middleware"
	"github.com/labring/aiproxy/core/model"
	"github.com/labring/aiproxy/core/router"
	log "github.com/sirupsen/logrus"
)

var listen string

func init() {
	flag.StringVar(&listen, "listen", ":3000", "http server listen")
}

func initializeServices() error {
	initializeNotifier()

	if err := initializeBalance(); err != nil {
		return err
	}

	if err := initializeDatabases(); err != nil {
		return err
	}

	return initializeCaches()
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

var logCallerIgnoreFuncs = map[string]struct{}{
	"github.com/labring/aiproxy/core/middleware.logColor": {},
}

func setLog(l *log.Logger) {
	gin.ForceConsoleColor()
	if config.DebugEnabled {
		l.SetLevel(log.DebugLevel)
		l.SetReportCaller(true)
		gin.SetMode(gin.DebugMode)
	} else {
		l.SetLevel(log.InfoLevel)
		l.SetReportCaller(false)
		gin.SetMode(gin.ReleaseMode)
	}
	l.SetOutput(os.Stdout)
	stdlog.SetOutput(l.Writer())

	l.SetFormatter(&log.TextFormatter{
		ForceColors:      true,
		DisableColors:    false,
		ForceQuote:       config.DebugEnabled,
		DisableQuote:     !config.DebugEnabled,
		DisableSorting:   false,
		FullTimestamp:    true,
		TimestampFormat:  time.DateTime,
		QuoteEmptyFields: true,
		CallerPrettyfier: func(f *runtime.Frame) (function, file string) {
			if _, ok := logCallerIgnoreFuncs[f.Function]; ok {
				return "", ""
			}
			return f.Function, fmt.Sprintf("%s:%d", f.File, f.Line)
		},
	})

	if common.NeedColor() {
		gin.ForceConsoleColor()
	}
}

func initializeDatabases() error {
	model.InitDB()
	model.InitLogDB()
	return common.InitRedisClient()
}

func initializeCaches() error {
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

func setupHTTPServer() (*http.Server, *gin.Engine) {
	server := gin.New()

	server.
		Use(middleware.GinRecoveryHandler).
		Use(middleware.NewLog(log.StandardLogger())).
		Use(middleware.RequestIDMiddleware, middleware.CORS())
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

func autoTestBannedModels(ctx context.Context) {
	log.Info("auto test banned models start")
	ticker := time.NewTicker(time.Second * 30)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			controller.AutoTestBannedModels()
		}
	}
}

func detectIPGroupsTask(ctx context.Context) {
	log.Info("detect IP groups start")
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if !trylock.Lock("detectIPGroups", time.Minute) {
				continue
			}
			detectIPGroups()
		}
	}
}

func detectIPGroups() {
	threshold := config.GetIPGroupsThreshold()
	if threshold < 1 {
		return
	}
	ipGroupList, err := model.GetIPGroups(int(threshold), time.Now().Add(-time.Hour), time.Now())
	if err != nil {
		notify.ErrorThrottle("detectIPGroups", time.Minute, "detect IP groups failed", err.Error())
	}
	if len(ipGroupList) == 0 {
		return
	}
	banThreshold := config.GetIPGroupsBanThreshold()
	for ip, groups := range ipGroupList {
		slices.Sort(groups)
		groupsJSON, err := sonic.MarshalString(groups)
		if err != nil {
			notify.ErrorThrottle(
				"detectIPGroupsMarshal",
				time.Minute,
				"marshal IP groups failed",
				err.Error(),
			)
			continue
		}

		if banThreshold >= threshold && len(groups) >= int(banThreshold) {
			rowsAffected, err := model.UpdateGroupsStatus(groups, model.GroupStatusDisabled)
			if err != nil {
				notify.ErrorThrottle(
					"detectIPGroupsBan",
					time.Minute,
					"update groups status failed",
					err.Error(),
				)
			}
			if rowsAffected > 0 {
				notify.Warn(
					fmt.Sprintf(
						"Suspicious activity: IP %s is using %d groups (exceeds ban threshold of %d). IP and all groups have been disabled.",
						ip,
						len(groups),
						banThreshold,
					),
					groupsJSON,
				)
				ipblack.SetIPBlackAnyWay(ip, time.Hour*48)
			}
			continue
		}

		h := sha256.New()
		h.Write(conv.StringToBytes(groupsJSON))
		groupsHash := hex.EncodeToString(h.Sum(nil))
		hashKey := fmt.Sprintf("%s:%s", ip, groupsHash)

		notify.WarnThrottle(
			hashKey,
			time.Hour*3,
			fmt.Sprintf(
				"Potential abuse: IP %s is using %d groups (exceeds threshold of %d)",
				ip,
				len(groups),
				threshold,
			),
			groupsJSON,
		)
	}
}

func cleanLog(ctx context.Context) {
	log.Info("clean log start")
	// the interval should not be too large to avoid cleaning too much at once
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if !trylock.Lock("cleanLog", time.Minute) {
				continue
			}
			optimize := trylock.Lock("optimizeLog", time.Hour*24)
			err := model.CleanLog(int(config.GetCleanLogBatchSize()), optimize)
			if err != nil {
				notify.ErrorThrottle("cleanLog", time.Minute, "clean log failed", err.Error())
			}
		}
	}
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

// Swagger godoc
//
//	@title						AI Proxy Swagger API
//	@version					1.0
//	@securityDefinitions.apikey	ApiKeyAuth
//	@in							header
//	@name						Authorization
func main() {
	flag.Parse()

	loadEnv()

	config.ReloadEnv()

	setLog(log.StandardLogger())

	printLoadedEnvFiles()

	if err := initializeServices(); err != nil {
		log.Fatal("failed to initialize services: " + err.Error())
	}

	defer func() {
		if err := model.CloseDB(); err != nil {
			log.Fatal("failed to close database: " + err.Error())
		}
	}()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	var wg sync.WaitGroup
	startSyncServices(ctx, &wg)

	srv, _ := setupHTTPServer()

	go func() {
		log.Infof("server started on %s", srv.Addr)
		log.Infof("swagger server started on %s/swagger/index.html", srv.Addr)
		if err := srv.ListenAndServe(); err != nil &&
			!errors.Is(err, http.ErrServerClosed) {
			log.Fatal("failed to start HTTP server: " + err.Error())
		}
	}()

	go autoTestBannedModels(ctx)
	go cleanLog(ctx)
	go detectIPGroupsTask(ctx)
	go controller.UpdateChannelsBalance(time.Minute * 10)

	batchProcessorCtx, batchProcessorCancel := context.WithCancel(context.Background())
	wg.Add(1)
	go model.StartBatchProcessorSummary(batchProcessorCtx, &wg)

	<-ctx.Done()

	shutdownSrvCtx, shutdownSrvCancel := context.WithTimeout(context.Background(), 600*time.Second)
	defer shutdownSrvCancel()

	log.Info("shutting down http server...")
	log.Info("max wait time: 600s")
	if err := srv.Shutdown(shutdownSrvCtx); err != nil {
		log.Error("server forced to shutdown: " + err.Error())
	} else {
		log.Info("server shutdown successfully")
	}

	log.Info("shutting down consumer...")
	consume.Wait()

	batchProcessorCancel()

	log.Info("shutting down sync services...")
	wg.Wait()

	log.Info("shutting down batch summary...")
	log.Info("max wait time: 600s")
	cleanCtx, cleanCancel := context.WithTimeout(context.Background(), 600*time.Second)
	defer cleanCancel()
	model.CleanBatchUpdatesSummary(cleanCtx)

	log.Info("server exiting")
}
