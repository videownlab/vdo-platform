package auth

import (
	"fmt"
	"testing"
	"vdo-platform/pkg/setting"

	"github.com/stretchr/testify/assert"
)

func TestParseTemplate(t *testing.T) {
	templateData := struct {
		AuthCode string
	}{
		AuthCode: "888888",
	}
	mailBody, err := parseTemplate(templateData)
	assert.NoError(t, err)
	fmt.Println(*mailBody)
}

func TestSendAuthCodeEmail(t *testing.T) {
	settings, err := setting.NewSettingsWithDirectory("../../../config")
	assert.NoError(t, err)
	err = sendAuthCodeEmail(settings.SmtpSetting, "666666", "0xdadak@proton.me")
	assert.NoError(t, err)
}
