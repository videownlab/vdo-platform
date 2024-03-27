package setting

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	l "vdo-platform/pkg/log"

	"github.com/spf13/viper"
)

type Settings struct {
	ServerSetting   *ServerSettingS
	AppSetting      *AppSettingS
	Web3Setting     *Web3SettingS
	DatabaseSetting *DatabaseSettingS
	RedisSetting    *RedisSettingS
	SmtpSetting     *SmtpSettingS
	vp              *viper.Viper
}

func (t *Settings) ChangePassword(newPassword, oldPassword string) error {
	if len(newPassword) < 8 {
		return fmt.Errorf("the new password length must greate and equal than 8")
	}
	if t.AppSetting.Password != oldPassword {
		return fmt.Errorf("the old password is invalid")
	}
	t.AppSetting.Password = newPassword
	t.vp.Set("app.password", newPassword)
	if err := t.vp.WriteConfig(); err != nil {
		return err
	}
	return nil
}

const (
	_CONFIG_FILENAME = "config.yaml"
)

func getConfigFilePathByEnv(configDir string) (string, error) {
	configFilePaths := make([]string, 0, 2)
	appEnv := os.Getenv("APP_ENV")
	if appEnv != "" {
		configFilePaths = append(configFilePaths, filepath.Join(configDir, "conf-"+strings.ToLower(appEnv)+".yaml"))
	}
	configFilePaths = append(configFilePaths, filepath.Join(configDir, _CONFIG_FILENAME))
	for _, cfp := range configFilePaths {
		_, err := os.Stat(cfp)
		if err == nil {
			return cfp, nil
		}
	}
	return "", fmt.Errorf("don't exist any config file: %s", strings.Join(configFilePaths, ", "))
}

func parseConfigNameAndType(configFilePath string) (n string, t string) {
	fn := filepath.Base(configFilePath)
	ext := filepath.Ext(fn)
	return fn[:len(fn)-len(ext)], strings.TrimPrefix(ext, ".")
}

func loadConfigFile(configDir string) (*viper.Viper, error) {
	var configName, configType string
	{
		configFilePath, err := getConfigFilePathByEnv(configDir)
		if err != nil {
			return nil, err
		}
		l.Logger.Info("use config", "configFile", configFilePath)
		configName, configType = parseConfigNameAndType(configFilePath)
	}

	vp := viper.New()
	vp.SetConfigName(configName)
	vp.AddConfigPath(configDir)
	vp.SetConfigType(configType)
	err := vp.ReadInConfig()
	if err != nil {
		return nil, err
	}
	return vp, nil
}

func NewSettings() (*Settings, error) {
	return NewSettingsWithDirectory("configs/")
}

func NewSettingsWithDirectory(configDir string) (*Settings, error) {
	if configDir == "" {
		configDir = "config/"
	}
	vp, err := loadConfigFile(configDir)
	if err != nil {
		return nil, err
	}

	pSettings := new(Settings)
	pSettings.vp = vp
	if err := vp.UnmarshalKey("Server", &pSettings.ServerSetting); err != nil {
		return nil, err
	}
	pSettings.ServerSetting.ReadTimeout *= time.Second
	pSettings.ServerSetting.WriteTimeout *= time.Second

	if err := vp.UnmarshalKey("App", &pSettings.AppSetting); err != nil {
		return nil, err
	}
	if err := vp.UnmarshalKey("Database", &pSettings.DatabaseSetting); err != nil {
		return nil, err
	}
	if err := vp.UnmarshalKey("Web3", &pSettings.Web3Setting); err != nil {
		return nil, err
	}
	if err := vp.UnmarshalKey("Redis", &pSettings.RedisSetting); err != nil {
		return nil, err
	}
	if err := vp.UnmarshalKey("Smtp", &pSettings.SmtpSetting); err != nil {
		return nil, err
	}

	return pSettings, nil
}
