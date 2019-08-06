package config

import (
	"fmt"

	"github.com/quite/pjodd/plugins/githook"
	"github.com/quite/pjodd/util"
)

type Config struct {
	IRC     IRC
	Githook githook.Githook
}

type IRC struct {
	Server []Server
}

type Server struct {
	Server   string
	SSL      bool
	Password string
	Nick     string
	Ident    string
	RealName string
	Channels []string
}

func (cfg *Config) Validate() error {
	if len(cfg.IRC.Server) == 0 {
		return fmt.Errorf("no IRC servers configured")
	}

	srvChans := make(map[string][]string)

	for i, s := range cfg.IRC.Server {
		if s.Server == "" {
			return fmt.Errorf("server (host:port) empty")
		}
		if s.Nick == "" {
			return fmt.Errorf("server `%s` has no nick", s.Server)
		}
		if s.Ident == "" {
			cfg.IRC.Server[i].Ident = s.Nick
		}
		if s.RealName == "" {
			cfg.IRC.Server[i].RealName = s.Nick
		}
		if len(s.Channels) == 0 {
			return fmt.Errorf("server `%s` has no channels", s.Server)
		}
		if _, ok := srvChans[s.Server]; ok {
			return fmt.Errorf("duplicate IRC server: %s", s.Server)
		}
		srvChans[s.Server] = s.Channels
	}

	if cfg.Githook.ListenAddr == "" {
		return fmt.Errorf("githook http listen host:port not configured")
	}

	if len(cfg.Githook.Target) == 0 {
		return fmt.Errorf("no githook targets configured")
	}

	for _, t := range cfg.Githook.Target {
		chans, ok := srvChans[t.Server]
		if !ok {
			return fmt.Errorf("githook target server not configured: %s", t.Server)
		}
		if !util.Contains(chans, t.Channel) {
			return fmt.Errorf("githook target server `%s` no configured for channel: %s", t.Server, t.Channel)
		}
	}
	return nil
}
