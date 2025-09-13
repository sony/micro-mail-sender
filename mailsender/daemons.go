package mailsender

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"regexp"
	"time"

	"go.uber.org/zap"
)

const (
	PostConfig     = "/etc/postfix/main.cf"
	PostConfigOrig = "/etc/postfix/main.cf.orig"
	SaslPasswd     = "/etc/postfix/sasl_passwd"
	PostOrigin     = "/etc/mailname"
)

// Only called in standalone mode (inside docker container).
// User must be root.
// Returns true on success, false on failure
func StartDaemons(config *Config) bool {
	logger, err := zap.NewDevelopment()
	if err != nil {
		log.Printf("can't initialize zap logger: %v", err)
		return false
	}
	sugar := logger.Sugar()

	if !runSyslog(sugar) {
		return false
	}
	if !runPostfix(sugar, config) {
		return false
	}
	return true
}

func runSyslog(sugar *zap.SugaredLogger) bool {
	return runExternalCommand(sugar, []string{"service", "rsyslog", "start"})
}

func runPostfix(sugar *zap.SugaredLogger, config *Config) bool {
	err := os.WriteFile(PostOrigin, []byte(config.MyDomain), 0644)
	if err != nil {
		sugar.Errorw("Cannot set up mailname",
			"path", PostOrigin,
			"error", err)
		return false
	}

	content, err := os.ReadFile(PostConfigOrig)
	if err != nil {
		sugar.Errorw("Cannot read mail.cf.orig",
			"path", PostConfigOrig,
			"error", err)
		return false
	}
	edited := regexp.MustCompile(`tmp\.local`).
		ReplaceAllString(string(content), config.MyDomain)
	if config.RelayHost != "" {
		edited = regexp.MustCompile(`relayhost = `).ReplaceAllString(edited, fmt.Sprintf("relayhost = %s", config.RelayHost))
	}
	if config.RelayUser != "" {
		if err = os.WriteFile(SaslPasswd, []byte(fmt.Sprintf("%s %s:%s\n", config.RelayHost, config.RelayUser, config.RelayPass)), 0600); err != nil {
			sugar.Errorw("Cannot write sasl_passwd",
				"path", SaslPasswd,
				"error", err)
			return false
		}
		if !runExternalCommand(sugar, []string{"postmap", "hash:/etc/postfix/sasl_passwd"}) {
			return false
		}
		if !runExternalCommand(sugar, []string{"apt-get", "update"}) {
			return false
		}
		if !runExternalCommand(sugar, []string{"apt-get", "install", "-y", "libsasl2-modules"}) {
			return false
		}
	}
	// Append additional configuration to PostConfig
	for key, value := range config.Others {
		edited += fmt.Sprintf("%s = %s\n", key, value)
	}

	err = os.WriteFile(PostConfig, []byte(edited), 0644)
	if err != nil {
		sugar.Errorw("Cannot write mail.cf",
			"path", PostConfig,
			"error", err)
		return false
	}

	if !runExternalCommand(sugar, []string{"postfix", "start"}) {
		return false
	}

	// restart postfix to workaround error
	// https://bytefreaks.net/gnulinux/host-or-domain-name-not-found-name-service-error-for-namesmtp-gmail-com-typeaaaa-host-not-found-try-again
	time.Sleep(10 * time.Second)
	runExternalCommand(zap.NewNop().Sugar(), []string{"service", "postfix", "restart"}) // ignore errors
	return true
}

func runExternalCommand(sugar *zap.SugaredLogger, argv []string) bool {
	cmd := exec.Command(argv[0], argv[1:]...)
	output, err := cmd.CombinedOutput()

	if err != nil {
		sugar.Errorw("Command execution failed",
			"command", argv,
			"error", err,
			"output", string(output))
		return false
	}

	sugar.Infow("Command executed successfully",
		"command", argv,
		"output", string(output))
	return true
}
