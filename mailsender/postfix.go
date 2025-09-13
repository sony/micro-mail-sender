package mailsender

import "os/exec"

// InitPostfix reconfigures postfix according to the Config.
// This is only used in container mode.
func InitPostfix(config *Config) error {
	cmd := exec.Command("/scripts/config-postfix.sh",
		config.MyDomain)
	return cmd.Run()
}
