package main

import (
	"flag"
	"fmt"
	"log"
	"sync"

	"github.com/BurntSushi/toml"
	"github.com/StalkR/goircbot/bot"
	"github.com/fluffle/goirc/logging/glog"
	"github.com/quite/sparv/config"
)

func main() {
	cfg := config.Config{}

	configflag := flag.String("config", "config.toml", "config file")
	flag.Parse()
	glog.Init()

	if _, err := toml.DecodeFile(*configflag, &cfg); err != nil {
		log.Fatal(err)
	}

	if err := cfg.Validate(); err != nil {
		log.Fatal(fmt.Errorf("invalid config: %s", err))
	}

	var wg sync.WaitGroup
	wg.Add(len(cfg.IRC.Server))

	bots := make(map[string]bot.Bot)
	for _, s := range cfg.IRC.Server {
		go func(s config.Server) {
			defer wg.Done()
			log.Printf("irc server:%s nick:%s channels:%v",
				s.Server, s.Nick, s.Channels)

			b, err := bot.NewBotOptions(bot.Host(s.Server), bot.Nick(s.Nick),
				bot.SSL(s.SSL), bot.Ident(s.Ident), bot.RealName(s.RealName),
				bot.Password(s.Password), bot.Channels(s.Channels))
			if err != nil {
				panic(err)
			}

			bots[s.Server] = b
			b.Run()
		}(s)
	}

	cfg.Githook.Listen(bots)

	wg.Wait()
}
