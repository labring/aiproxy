package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
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
	"github.com/labring/aiproxy/core/common/pprof"
	"github.com/labring/aiproxy/core/common/trylock"
	"github.com/labring/aiproxy/core/controller"
	"github.com/labring/aiproxy/core/middleware"
	"github.com/labring/aiproxy/core/model"
	"github.com/labring/aiproxy/core/router"
	log "github.com/sirupsen/logrus"
)

var (
	listen    string
	pprofPort int
)

func init() {
	flag.StringVar(&listen, "listen", "0.0.0.0:3000", "http server listen")
	flag.IntVar(&pprofPort, "pprof-port", 15000, "pport http server port")
}

func initializeServices() error {
	initializePprof()
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

func initializePprof() {
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

func setupHTTPServer() (*http.Server, *gin.Engine) {
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

func autoTestBannedModels(ctx context.Context) {
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
	// the interval should not be too large to avoid cleaning too much at once
	ticker := time.NewTicker(time.Second * 5)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if !trylock.Lock("cleanLog", time.Second*5) {
				continue
			}

			optimize := trylock.Lock("optimizeLog", time.Hour*24)

			err := model.CleanLog(int(config.GetCleanLogBatchSize()), optimize)
			if err != nil {
				notify.ErrorThrottle("cleanLog", time.Minute*5, "clean log failed", err.Error())
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

func listenAndServe(srv *http.Server) {
	if err := srv.ListenAndServe(); err != nil &&
		!errors.Is(err, http.ErrServerClosed) {
		log.Fatal("failed to start HTTP server: " + err.Error())
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

	common.InitLog(log.StandardLogger(), config.DebugEnabled)

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

	log.Info("auto test banned models task started")

	go autoTestBannedModels(ctx)

	log.Info("clean log task started")

	go cleanLog(ctx)

	log.Info("detect ip groups task started")

	go detectIPGroupsTask(ctx)

	log.Info("update channels balance task started")

	go controller.UpdateChannelsBalance(time.Minute * 10)

	batchProcessorCtx, batchProcessorCancel := context.WithCancel(context.Background())

	wg.Add(1)

	go model.StartBatchProcessorSummary(batchProcessorCtx, &wg)

	log.Infof("server started on http://%s", srv.Addr)
	log.Infof("swagger started on http://%s/swagger/index.html", srv.Addr)

	go listenAndServe(srv)

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
