package service

import (
	"bufio"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// startCaptureSMTPServer accepts one plain SMTP session and returns the DATA
// body it received. It is intentionally minimal: the project's SMTP client only
// needs the standard greeting/command responses for a local delivery.
func startCaptureSMTPServer(t *testing.T) (addr string, received <-chan string) {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { _ = listener.Close() })

	bodyCh := make(chan string, 1)
	go func() {
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		reader := bufio.NewReader(conn)
		writer := bufio.NewWriter(conn)
		reply := func(line string) {
			_, _ = writer.WriteString(line + "\r\n")
			_ = writer.Flush()
		}
		reply("220 localhost capture")
		inData := false
		var body strings.Builder
		for {
			line, err := reader.ReadString('\n')
			if err != nil {
				break
			}
			line = strings.TrimRight(line, "\r\n")
			if inData {
				if line == "." {
					inData = false
					bodyCh <- body.String()
					reply("250 accepted")
					continue
				}
				body.WriteString(line)
				body.WriteString("\n")
				continue
			}
			switch {
			case strings.HasPrefix(line, "EHLO"), strings.HasPrefix(line, "HELO"):
				reply("250-localhost")
				reply("250 OK")
			case strings.HasPrefix(line, "MAIL FROM"):
				reply("250 OK")
			case strings.HasPrefix(line, "RCPT TO"):
				reply("250 OK")
			case line == "DATA":
				reply("354 send data")
				inData = true
			case line == "QUIT":
				reply("221 bye")
				return
			default:
				reply("250 OK")
			}
		}
	}()
	return listener.Addr().String(), bodyCh
}

func TestJailbreakWarningEmailSendsWarningOrdinal(t *testing.T) {
	setupJailbreakTest(t)
	user := seedJailbreakUser(t, common.RoleCommonUser, common.UserStatusEnabled)
	user.Email = "warned-user@example.com"
	require.NoError(t, model.DB.Model(&model.User{}).Where("id = ?", user.Id).
		Update("email", user.Email).Error)

	addr, received := startCaptureSMTPServer(t)
	host, portText, err := net.SplitHostPort(addr)
	require.NoError(t, err)
	port, err := parsePort(portText)
	require.NoError(t, err)

	previousServer, previousPort := common.SMTPServer, common.SMTPPort
	previousAccount, previousToken := common.SMTPAccount, common.SMTPToken
	previousFrom := common.SMTPFrom
	common.SMTPServer = host
	common.SMTPPort = port
	common.SMTPAccount = ""
	common.SMTPToken = ""
	common.SMTPFrom = "noreply@example.com"
	t.Cleanup(func() {
		common.SMTPServer = previousServer
		common.SMTPPort = previousPort
		common.SMTPAccount = previousAccount
		common.SMTPToken = previousToken
		common.SMTPFrom = previousFrom
	})

	sendJailbreakWarningEmail(user.Id, 2, 3)

	select {
	case body := <-received:
		assert.Contains(t, body, "warning 2 of 3")
		assert.Contains(t, body, "Remaining warnings: 1")
	case <-time.After(10 * time.Second):
		t.Fatal("warning email was not delivered to the capture SMTP server")
	}
}

func parsePort(text string) (int, error) {
	port := 0
	for _, r := range text {
		if r < '0' || r > '9' {
			return 0, assert.AnError
		}
		port = port*10 + int(r-'0')
	}
	return port, nil
}
