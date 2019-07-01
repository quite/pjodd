// Go IRC Bot example.
package main

import (
	"flag"
	"fmt"
	"log"

	"github.com/BurntSushi/toml"
	"github.com/quite/pjodd/plugins/githook"

	"github.com/StalkR/goircbot/bot"
	"github.com/fluffle/goirc/logging/glog"
)

type config struct {
	Server   string
	Port     int
	Ssl      bool
	Nickname string
	Username string
	Realname string
	Channels []string
	Githook  githook.Githook
}

func (cfg *config) validate() error {
	return cfg.Githook.Validate(cfg.Channels)
}

func main() {
	cfg := config{
		Server:   "localhost",
		Port:     6667,
		Ssl:      false,
		Nickname: "pjodd",
		Username: "pjodd",
		Realname: "Pjodd", // unused
		Channels: []string{"#test"},
	}

	configflag := flag.String("config", "config.toml", "config file")
	flag.Parse()
	glog.Init()

	if _, err := toml.DecodeFile(*configflag, &cfg); err != nil {
		log.Fatal(err)
	}
	if err := cfg.validate(); err != nil {
		log.Fatal(fmt.Errorf("error validating Githook config: %s", err))
	}

	log.Printf("server:%s:%d nick:%s channels:%v",
		cfg.Server, cfg.Port, cfg.Nickname, cfg.Channels)

	b := bot.NewBot(fmt.Sprintf("%s:%d", cfg.Server, cfg.Port),
		cfg.Ssl, cfg.Nickname, cfg.Username, cfg.Channels)

	cfg.Githook.Listen(b)

	b.Run()
}
