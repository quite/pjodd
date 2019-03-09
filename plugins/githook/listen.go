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

func push(push github.PushPayload) []string {
	lines := []string{}

	l := ""
	l += fmt.Sprintf("[%s]", push.Repository.Name)
	l += fmt.Sprintf(" %s", push.Pusher.Name)

	l += " "
	if push.Forced {
		l += "force-"
	}
	l += "pushed"
	l += fmt.Sprintf(" %d commit", len(push.Commits))
	if len(push.Commits) > 1 {
		l += fmt.Sprintf("s")
	}

	ss := strings.Split(push.Ref, "/")
	branch := ss[len(ss)-1]
	l += fmt.Sprintf(" to %s", branch)

	long := ""
	if len(push.Commits) == 1 {
		long = fmt.Sprintf("https://github.com/%s/commit/%s",
			push.Repository.FullName, push.HeadCommit.ID)
	} else {
		long = fmt.Sprintf("https://github.com/%s/compare/%s...%s",
			push.Repository.FullName, push.Before, push.After)
	}
	l += fmt.Sprintf(": %s", shorten(long))
	lines = append(lines, l)

	first := len(push.Commits) - 3
	if first < 0 {
		first = 0
	}
	last := len(push.Commits) - 1
	if last < 0 {
		last = 0
	}
	for i := last; i >= first; i-- {
		c := push.Commits[i]
		ss = strings.Split(c.Message, "\n")
		l := fmt.Sprintf("%s/%s %s %s: %s",
			push.Repository.Name, branch, c.ID[:7], c.Committer.Name, ss[0])
		lines = append(lines, l)
	}
	return lines
}

func listen(hostport string, path string, secret string, notify func(string)) {
	hook, _ := github.New(github.Options.Secret(secret))
	http.HandleFunc(path, func(w http.ResponseWriter, r *http.Request) {
		payload, err := hook.Parse(r, github.PushEvent)
		if err != nil {
			log.Printf("hook.Parse: %s\n", err)
			return
		}
		switch payload.(type) {

		case github.PushPayload:
			lines := push(payload.(github.PushPayload))
			for _, l := range lines {
				notify(l)
			}
		case github.PingPayload:
			ping := payload.(github.PingPayload)
			fmt.Printf("%+v\n", ping)
		}
	})
	http.ListenAndServe(hostport, nil)
}

func notify(b bot.Bot, channel string, line string) {
	if !b.Connected() {
		return
	}
	for _, onchan := range b.Channels() {
		if onchan == channel {
			b.Privmsg(channel, line)
			return
		}
	}
	log.Printf("notify: I'm not on channel %s\n", channel)
}

func Listen(b bot.Bot, hostport string, path string, secret string, channel string) {
	log.Printf("channel %s : github webhook listening %s%s", channel, hostport, path)
	go listen(hostport, path, secret, func(line string) {
		notify(b, channel, line)
	})
}
