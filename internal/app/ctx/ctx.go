package ctx

import (
	"vdo-platform/pkg/chain"
	"vdo-platform/pkg/setting"

	"gorm.io/gorm"
)

const COVER_IMAGE_PATH = "./cover_images/"
const DEFAULT_COVER_IMAGE = "./cover_images/default.png"

var Time_FMT = "2006-01-02 15:04:05"
var Settings *setting.Settings
var GormDb *gorm.DB
var ChainClient *chain.ChainClient
