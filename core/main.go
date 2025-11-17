package main

import (
	"context"
	"flag"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/labring/aiproxy/core/common"
	"github.com/labring/aiproxy/core/common/config"
	"github.com/labring/aiproxy/core/common/consume"
	"github.com/labring/aiproxy/core/controller"
	"github.com/labring/aiproxy/core/model"
	"github.com/labring/aiproxy/core/task"
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

	if err := ensureAdminKey(); err != nil {
		log.Warn("failed to ensure AdminKey: " + err.Error())
	}

	common.InitLog(log.StandardLogger(), config.DebugEnabled)

	printLoadedEnvFiles()

	if err := initializeServices(pprofPort); err != nil {
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

	srv, _ := setupHTTPServer(listen)

	log.Info("auto test banned models task started")

	go task.AutoTestBannedModelsTask(ctx)

	log.Info("clean log task started")

	go task.CleanLogTask(ctx)

	log.Info("detect ip groups task started")

	go task.DetectIPGroupsTask(ctx)

	log.Info("usage alert task started")

	go task.UsageAlertTask(ctx)

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
