package app

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
	"vdo-platform/internal/app/ctx"
	"vdo-platform/internal/app/db"
	"vdo-platform/internal/ginlet"
	"vdo-platform/internal/service"
	"vdo-platform/pkg/chain"
	"vdo-platform/pkg/setting"

	"github.com/gin-gonic/gin"
	"github.com/urfave/cli"
)

func Setup(c *cli.Context) {
	if err := setupSetting(c); err != nil {
		log.Fatalf("init.setupSetting err: %v", err)
		return
	}

	if err := setupLogger(); err != nil {
		log.Fatalf("init.setupLogger err: %v", err)
		return
	}

	if err := setupDbEngine(); err != nil {
		log.Fatalf("init.setupDBEngine err: %v", err)
		return
	}

	if err := setupChainConnection(); err != nil {
		log.Fatalf("init.setupChainConnection err: %v", err)
		return
	}

	service.Setup()

	setupGin()
	signalHandle()
}

type DefaultAdminVerifyProvider struct {
	settings *setting.Settings
}

func (t DefaultAdminVerifyProvider) Verify(username, password string) error {
	if username == t.settings.AppSetting.Username && password == t.settings.AppSetting.Password {
		return nil
	}
	return fmt.Errorf("invalid username or password")
}

func setupGin() {
	gin.SetMode(ctx.Settings.ServerSetting.RunMode)

	routerHandler := ginlet.NewRouter()
	httpServer := &http.Server{
		Addr:           ":" + ctx.Settings.ServerSetting.HttpPort,
		Handler:        routerHandler,
		ReadTimeout:    ctx.Settings.ServerSetting.ReadTimeout,
		WriteTimeout:   ctx.Settings.ServerSetting.WriteTimeout,
		MaxHeaderBytes: 1 << 20,
	}

	go func() {
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("listen: %s\n", err)
		}
	}()
}

func setupSetting(c *cli.Context) error {
	var err error
	configDir := c.String("config")
	if configDir != "" {
		ctx.Settings, err = setting.NewSettingsWithDirectory(configDir)

	} else {
		ctx.Settings, err = setting.NewSettings()
	}
	return err
}

func setupLogger() error {
	// ethlog.Root().SetHandler(ethlog.StdoutHandler)
	return nil
}

func setupDbEngine() error {
	var err error
	ctx.GormDb, err = db.NewGormDbForMySql(ctx.Settings.DatabaseSetting, ctx.Settings.ServerSetting.IsDebugMode())
	if err != nil {
		panic("database init err! " + err.Error())
	}
	return err
}

func setupChainConnection() error {
	// connecting chain
	var err error
	ctx.ChainClient, err = chain.NewChainClient(
		ctx.Settings.Web3Setting.RpcEndpoints[0],
		ctx.Settings.Web3Setting.Mnemonic,
		30*time.Second,
	)
	if err != nil {
		return err
	}
	// sync block
	for {
		ok, err := ctx.ChainClient.GetSyncStatus()
		if err != nil {
			return err
		}
		if !ok {
			break
		}
		fmt.Println("In sync block...")
		time.Sleep(time.Second * 10)
	}
	chain.InitRpcWorkPool()
	fmt.Println("Complete synchronization of primary network block data")
	fmt.Println("building chain success!")
	return nil
}

func signalHandle() {
	log.Println("vdo-platform server startup success!")
	var ch = make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGHUP, syscall.SIGQUIT, syscall.SIGTERM, syscall.SIGINT)
	for {
		si := <-ch
		switch si {
		case syscall.SIGQUIT, syscall.SIGTERM, syscall.SIGINT:
			log.Printf("get a signal: %s, stop the vdo-platform server process\n", si.String())
			log.Println("vdo-platform server shutdown success!")
			return
		case syscall.SIGHUP:
		default:
			return
		}
	}
}
