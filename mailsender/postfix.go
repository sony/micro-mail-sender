package mailsender

import (
	"os/exec"

	"github.com/cockroachdb/errors"
)

// InitPostfix reconfigures postfix according to the Config.
// This is only used in container mode.
func InitPostfix(config *Config) error {
	cmd := exec.Command("/scripts/config-postfix.sh", config.MyDomain) // #nosec G204
	return errors.WithStack(cmd.Run())
}
