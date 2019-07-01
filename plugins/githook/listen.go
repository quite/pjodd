package githook

import (
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strings"

	"github.com/StalkR/goircbot/bot"
	"gopkg.in/go-playground/webhooks.v5/github"
)

const (
	maxcommits = 3
)

// Only github for now
var types = []string{"github"}

type Githook struct {
	Host   string
	Port   int
	Target []Target
}

type Target struct {
	Type    string
	Path    string
	Secret  string
	Channel string
}

func contains(ss []string, s string) bool {
	for _, i := range ss {
		if i == s {
			return true
		}
	}
	return false
}

func (gh Githook) Validate(channels []string) error {
	if gh.Port == 0 {
		return fmt.Errorf("http listen port not configured (==0)")
	}
	if len(gh.Target) == 0 {
		return fmt.Errorf("no targets configured")
	}
	for _, target := range gh.Target {
		if !contains(types, target.Type) {
			return fmt.Errorf("target type `%s` is not one of %s",
				target.Type, types)
		}
		if !contains(channels, target.Channel) {
			return fmt.Errorf("target channel `%s` is not among the configured %s",
				target.Channel, channels)
		}
	}
	return nil
}

func (gh Githook) Listen(b bot.Bot) {
	go gh.doListen(
		func(ch string, line string) {
			if !b.Connected() {
				log.Printf("say: not connected")
				return
			}
			if !stringIn(ch, b.Channels()) {
				log.Printf("say: not on %s\n", ch)
				return
			}
			b.Privmsg(ch, line)
		},
		func(msg string) {
			b.Quit(msg)
		})
}

func (gh Githook) doListen(say func(string, string), quit func(string)) {
	for _, target := range gh.Target {
		// rebind to a new var for the closure
		target := target

		log.Printf("githook github target path:%s channel:%s\n",
			target.Path, target.Channel)
		hook, _ := github.New(github.Options.Secret(target.Secret))

		http.HandleFunc(target.Path, func(w http.ResponseWriter, r *http.Request) {
			payload, err := hook.Parse(r, github.PushEvent, github.PingEvent)
			if err != nil {
				log.Printf("hook.Parse: %s\n", err)
				return
			}
			switch payload := payload.(type) {
			case github.PushPayload:
				lines := buildPushLines(payload)
				log.Printf("path:%s channel:%s\n", target.Path, target.Channel)
				for _, l := range lines {
					say(target.Channel, l)
					log.Printf("  %s\n", l)
				}
			case github.PingPayload:
				log.Printf("pinged: %+v\n", payload)
			}
		})
	}

	log.Printf("githook listening on %s:%d\n", gh.Host, gh.Port)
	err := http.ListenAndServe(fmt.Sprintf("%s:%d", gh.Host, gh.Port), nil)
	if err != nil {
		// TODO be more graceful perhaps. quit at all?
		quit(err.Error())
		log.Fatal(err)
	}
}

func shorten(longurl string) string {
	resp, err := http.PostForm("https://git.io",
		url.Values{"url": {longurl}})
	if err != nil {
		return longurl
	}
	loc, ok := resp.Header["Location"]
	if !ok {
		return longurl
	}
	return loc[0]
}

func lastString(ss []string) string {
	return ss[len(ss)-1]
}

func stringIn(s string, ss []string) bool {
	for _, has := range ss {
		if has == s {
			return true
		}
	}
	return false
}

func buildPushLines(push github.PushPayload) []string {
	lines := []string{}

	repo := push.Repository.Name
	repofull := push.Repository.FullName
	pusher := push.Pusher.Name
	verb := "pushed"
	if push.Forced {
		verb = "force-" + verb
	}
	count := len(push.Commits)
	noun := "commit"
	if count > 1 {
		noun += "s"
	}
	branch := lastString(strings.Split(push.Ref, "/"))

	longurl := ""
	if count == 1 {
		longurl = fmt.Sprintf("https://github.com/%s/commit/%s",
			repofull, push.HeadCommit.ID)
	} else {
		longurl = fmt.Sprintf("https://github.com/%s/compare/%s...%s",
			repofull, push.Before, push.After)
	}

	l := fmt.Sprintf("[%s] %s %s %d %s to %s: %s",
		repo, pusher, verb, count, noun,
		branch, shorten(longurl))
	lines = append(lines, l)

	for i, n := count-1, maxcommits; i >= 0 && n > 0; i-- {
		c := push.Commits[i]
		subject := lastString(strings.Split(c.Message, "\n"))
		l := fmt.Sprintf("%s/%s %s %s: %s",
			repo, branch, c.ID[:7], c.Committer.Name, subject)
		lines = append(lines, l)
		n--
	}

	return lines
}
