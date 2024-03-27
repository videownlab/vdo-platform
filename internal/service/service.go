package service

import (
	"vdo-platform/internal/app/ctx"
	"vdo-platform/internal/model"
	"vdo-platform/internal/service/account"
	"vdo-platform/internal/service/auth"
	"vdo-platform/internal/service/nft"
	"vdo-platform/pkg/log"

	"gorm.io/gorm"
)

var AccountService *account.AccountService

func Setup() {
	autoMigrate(ctx.GormDb)

	AccountService = account.NewService(ctx.GormDb, ctx.Settings.Web3Setting.ChainId, log.Logger)
	auth.Setup(ctx.Settings, AccountService, log.Logger)
	nft.Setup()
}

func autoMigrate(db *gorm.DB) {
	account.AutoMigrate(db)
	model.AutoMigrate(db)
}
