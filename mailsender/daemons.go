package mailsender

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"regexp"
	"time"

	"github.com/cockroachdb/errors"
	"go.uber.org/zap"
)

const (
	// PostConfig path of main.cf
	PostConfig = "/etc/postfix/main.cf"
	// PostConfigOrig path of main.cf.orig
	PostConfigOrig = "/etc/postfix/main.cf.orig"
	// SaslPasswd path of sasl_passwd
	SaslPasswd = "/etc/postfix/sasl_passwd" // #nosec G101
	// PostOrigin path of mailname
	PostOrigin = "/etc/mailname"
)

// StartDaemons Only called in standalone mode (inside docker container).
// User must be root.
// Returns true on success, false on failure
func StartDaemons(config *Config) bool {
	logger, err := createLogger()
	if err != nil {
		log.Printf("cannot initialize logger: %+v", err)
		return false
	}

	if !runSyslog(logger) {
		return false
	}

	if !runPostfix(logger, config) {
		return false
	}

	return true
}

func runSyslog(logger *zap.SugaredLogger) bool {
	return runExternalCommand(logger, []string{"service", "rsyslog", "start"})
}

var tmpLocalRegexp = regexp.MustCompile(`tmp\.local`)
var relayhostRegexp = regexp.MustCompile(`relayhost = `)

func runPostfix(logger *zap.SugaredLogger, config *Config) bool {
	err := os.WriteFile(PostOrigin, []byte(config.MyDomain), 0644) // #nosec G306
	if err != nil {
		logger.Errorf("cannot set up mailname: %s %+v", PostOrigin, errors.WithStack(err))
		return false
	}

	content, err := os.ReadFile(PostConfigOrig)
	if err != nil {
		logger.Errorf("cannot read mail.cf.orig: %s %+v", PostConfigOrig, errors.WithStack(err))
		return false
	}

	edited := tmpLocalRegexp.ReplaceAllString(string(content), config.MyDomain)
	if config.RelayHost != "" {
		edited = relayhostRegexp.ReplaceAllString(edited, fmt.Sprintf("relayhost = %s", config.RelayHost))
	}
	if config.RelayUser != "" {
		if err = os.WriteFile(SaslPasswd, []byte(fmt.Sprintf("%s %s:%s\n", config.RelayHost, config.RelayUser, config.RelayPass)), 0600); err != nil {
			logger.Errorf("cannot write sasl_passwd: %+v", errors.WithStack(err))
			return false
		}
		if !runExternalCommand(logger, []string{"postmap", "hash:/etc/postfix/sasl_passwd"}) {
			return false
		}
		if !runExternalCommand(logger, []string{"apt-get", "update"}) {
			return false
		}
		if !runExternalCommand(logger, []string{"apt-get", "install", "-y", "libsasl2-modules"}) {
			return false
		}
	}
	// Append additional configuration to PostConfig
	for key, value := range config.Others {
		edited += fmt.Sprintf("%s = %s\n", key, value)
	}

	err = os.WriteFile(PostConfig, []byte(edited), 0644) // #nosec G306
	if err != nil {
		logger.Errorf("cannot write mail.cf: %s %+v", PostConfig, errors.WithStack(err))
		return false
	}

	if !runExternalCommand(logger, []string{"postfix", "start"}) {
		return false
	}

	// restart postfix to workaround error
	// https://bytefreaks.net/gnulinux/host-or-domain-name-not-found-name-service-error-for-namesmtp-gmail-com-typeaaaa-host-not-found-try-again
	time.Sleep(10 * time.Second)
	runExternalCommand(zap.NewNop().Sugar(), []string{"service", "postfix", "restart"}) // ignore errors
	return true
}

func runExternalCommand(logger *zap.SugaredLogger, argv []string) bool {
	cmd := exec.Command(argv[0], argv[1:]...) // #nosec G204
	output, err := cmd.CombinedOutput()

	if err != nil {
		logger.Errorf("command execution failed argv: %+v err: %+v output: %s", argv, err, string(output))
		return false
	}

	logger.Infow("Command executed successfully",
		"command", argv,
		"output", string(output))
	return true
}
