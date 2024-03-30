package auth

import (
	"bytes"
	"embed"
	"fmt"
	"html/template"
	"vdo-platform/pkg/setting"

	"gopkg.in/gomail.v2"
)

//go:embed auth_code_email_template.html
var templateFS embed.FS

func sendAuthCodeEmail(smtpSetting *setting.SmtpSettingS, autoCode string, to ...string) error {
	templateData := struct {
		AuthCode string
	}{
		AuthCode: autoCode,
	}
	subject := fmt.Sprintf("[%s] Videown auth code", templateData.AuthCode)
	mailBody, err := parseTemplate(templateData)
	if err != nil {
		return err
	}

	m := gomail.NewMessage()
	m.SetHeader("From", smtpSetting.FromEmail)
	m.SetHeader("To", to...)
	m.SetHeader("Subject", subject)
	m.SetBody("text/html", *mailBody)

	// mail.NetDialTimeout = func(network string, address string, timeout time.Duration) (net.Conn, error) {
	// 	dial, err := proxy.SOCKS5("tcp", "127.0.0.1:7890", nil, proxy.Direct)
	// 	if err != nil {
	// 		return nil, err
	// 	}
	// 	return dial.Dial("tcp", smtp_host.(string))
	// }

	d := gomail.NewDialer(smtpSetting.Host, int(smtpSetting.Port), smtpSetting.Username, smtpSetting.Password)
	return d.DialAndSend(m)
}

func parseTemplate(data interface{}) (*string, error) {
	t, err := template.ParseFS(templateFS, "auth_code_email_template.html")
	if err != nil {
		return nil, err
	}
	buf := new(bytes.Buffer)
	if err = t.Execute(buf, data); err != nil {
		return nil, err
	}
	body := buf.String()
	return &body, nil
}
