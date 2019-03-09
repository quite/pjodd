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

// only github so far...

const (
	maxcommits = 3
)

type Githook struct {
	Server  string
	Port    int
	Path    string
	Secret  string
	Channel string
}

func (gh Githook) Listen(b bot.Bot) {
	log.Printf("channel %s : github webhook listening %s:%d%s",
		gh.Channel, gh.Server, gh.Port, gh.Path)

	go gh.doListen(func(line string) {
		if !b.Connected() {
			log.Printf("notify: not connected")
			return
		}
		if !stringIn(gh.Channel, b.Channels()) {
			log.Printf("notify: not on %s\n", gh.Channel)
			return
		}
		b.Privmsg(gh.Channel, line)
	})
}

func (gh Githook) doListen(notify func(string)) {
	hook, _ := github.New(github.Options.Secret(gh.Secret))
	http.HandleFunc(gh.Path, func(w http.ResponseWriter, r *http.Request) {
		payload, err := hook.Parse(r, github.PushEvent)
		if err != nil {
			log.Printf("hook.Parse: %s\n", err)
			return
		}
		switch payload.(type) {

		case github.PushPayload:
			lines := buildPushLines(payload.(github.PushPayload))
			for _, l := range lines {
				notify(l)
			}
		case github.PingPayload:
			ping := payload.(github.PingPayload)
			fmt.Printf("%+v\n", ping)
		}
	})
	http.ListenAndServe(fmt.Sprintf("%s:%d", gh.Server, gh.Port), nil)
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

func abs(i int) int {
	if i < 0 {
		i = -i
	}
	return i
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

	for i := count - 1; i >= 0; i-- {
		if i >= abs(count-maxcommits) && i <= abs(count-1) {
			c := push.Commits[i]
			subject := lastString(strings.Split(c.Message, "\n"))
			l := fmt.Sprintf("%s/%s %s %s: %s",
				repo, branch, c.ID[:7], c.Committer.Name, subject)
			lines = append(lines, l)
		}
	}

	return lines
}
