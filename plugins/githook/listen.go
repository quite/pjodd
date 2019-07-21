package githook

import (
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strings"

	"github.com/StalkR/goircbot/bot"
	"gopkg.in/go-playground/webhooks.v5/github"
	"gopkg.in/go-playground/webhooks.v5/gitlab"
)

const (
	maxcommits = 3
)

type Githook struct {
	Host   string
	Port   int
	Target []Target
}

type Target struct {
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

		http.HandleFunc(target.Path, func(w http.ResponseWriter, r *http.Request) {
			_, fromGithub := r.Header["X-Github-Event"]
			_, fromGitlab := r.Header["X-Gitlab-Event"]
			switch {
			case fromGithub:
				hook, _ := github.New(github.Options.Secret(target.Secret))
				payload, err := hook.Parse(r, github.PushEvent, github.PingEvent)
				if err != nil {
					log.Printf("github hook.Parse: %s\n", err)
					return
				}
				switch payload := payload.(type) {
				case github.PushPayload:
					log.Printf("path:%s channel:%s\n", target.Path, target.Channel)
					pd := newPushDataFromGithub(payload)
					for _, l := range pd.buildPushLines() {
						say(target.Channel, l)
						log.Printf("  %s\n", l)
					}
				case github.PingPayload:
					log.Printf("pinged: %+v\n", payload)
				}
			case fromGitlab:
				hook, _ := gitlab.New(gitlab.Options.Secret(target.Secret))
				payload, err := hook.Parse(r, gitlab.PushEvents)
				if err != nil {
					log.Printf("gitlab hook.Parse: %s\n", err)
					log.Printf("  %#v\n", r.Header)
					return
				}
				switch payload := payload.(type) {
				case gitlab.PushEventPayload:
					log.Printf("path:%s channel:%s\n", target.Path, target.Channel)
					pd := newPushDataFromGitlab(payload)
					for _, l := range pd.buildPushLines() {
						say(target.Channel, l)
						log.Printf("  %s\n", l)
					}
					// case gitlab.PingPayload:
					//      log.Printf("pinged: %+v\n", payload)
				}
			default:
				log.Printf("webhooked from neither github nor gitlab")
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
	defer resp.Body.Close()

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

type pushData struct {
	repo     string
	repoFull string
	pusher   string
	verb     string
	count    int64
	branch   string
	longURL  string
	commits  []commitData
}
type commitData struct {
	message   string
	id        string
	committer string
}

func newPushDataFromGithub(push github.PushPayload) *pushData {
	pd := pushData{}
	pd.repo = push.Repository.Name
	pd.repoFull = push.Repository.FullName
	pd.pusher = push.Pusher.Name
	pd.verb = "pushed"
	if push.Forced {
		pd.verb = "force-" + pd.verb
	}
	pd.count = int64(len(push.Commits))
	pd.branch = lastString(strings.Split(push.Ref, "/"))
	if pd.count == 1 {
		pd.longURL = fmt.Sprintf("https://github.com/%s/commit/%s",
			pd.repoFull, push.HeadCommit.ID)
	} else {
		pd.longURL = fmt.Sprintf("https://github.com/%s/compare/%s...%s",
			pd.repoFull, push.Before, push.After)
	}
	for _, c := range push.Commits {
		pd.commits = append(pd.commits,
			commitData{
				message:   c.Message,
				id:        c.ID,
				committer: c.Committer.Name,
			})
	}
	return &pd
}

func newPushDataFromGitlab(push gitlab.PushEventPayload) *pushData {
	pd := pushData{}
	pd.repo = push.Project.Name
	pd.repoFull = push.Project.PathWithNamespace
	pd.pusher = push.UserName
	pd.verb = "pushed"
	// TODO no force-pushed?
	pd.count = push.TotalCommitsCount
	pd.branch = lastString(strings.Split(push.Ref, "/"))
	if pd.count == 1 {
		pd.longURL = push.Commits[0].URL
	} else {
		pd.longURL = fmt.Sprintf("%s/%s...%s",
			push.Project.WebURL, push.Before, push.After)
	}
	for _, c := range push.Commits {
		pd.commits = append(pd.commits,
			commitData{
				message:   c.Message,
				id:        c.ID,
				committer: c.Author.Name,
			})
	}
	return &pd
}

func (pd *pushData) buildPushLines() []string {
	lines := []string{}
	noun := "commit"
	if pd.count > 1 {
		noun += "s"
	}
	// TODO shorten longurl
	l := fmt.Sprintf("[%s] %s %s %d %s to %s: %s",
		pd.repo, pd.pusher, pd.verb, pd.count, noun,
		pd.branch, pd.longURL)
	lines = append(lines, l)

	for i, n := len(pd.commits)-1, maxcommits; i >= 0 && n > 0; i-- {
		c := pd.commits[i]
		l := fmt.Sprintf("%s/%s %s %s: %s",
			pd.repo, pd.branch, c.id[:7], c.committer, strings.Split(c.message, "\n")[0])
		lines = append(lines, l)
		n--
	}

	return lines
}
