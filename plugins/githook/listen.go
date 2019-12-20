package githook

import (
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/StalkR/goircbot/bot"
	"github.com/quite/sparv/util"
	"gopkg.in/go-playground/webhooks.v5/github"
	"gopkg.in/go-playground/webhooks.v5/gitlab"
)

const (
	maxcommits = 3
)

type Githook struct {
	ListenAddr string `toml:"listen"`
	Target     []Target
}

type Target struct {
	Path    string
	Secret  string
	Server  string
	Channel string
}

func (gh Githook) Listen(bots map[string]bot.Bot) {
	go gh.doListen(
		func(t Target, l string) {
			b, ok := bots[t.Server]
			if !ok {
				log.Printf("say: target bot `%s` not found", t.Server)
				return
			}
			if !b.Connected() {
				log.Printf("say: bot `%s` not connected", t.Server)
				return
			}
			if !util.Contains(b.Channels(), t.Channel) {
				log.Printf("say: bot `%s` not on chan `%s`\n", t.Server, t.Channel)
				return
			}
			b.Privmsg(t.Channel, l)
		},
		func(msg string) {
			for _, b := range bots {
				b.Quit(msg)
			}
		})
}

func (gh Githook) doListen(say func(Target, string), quit func(string)) {
	if gh.ListenAddr == "" || len(gh.Target) == 0 {
		return
	}

	for _, t := range gh.Target {
		// rebind to a new var for the closure
		t := t

		log.Printf("githook path:%s targeting %s/%s\n",
			t.Path, t.Server, t.Channel)

		http.HandleFunc(t.Path, func(w http.ResponseWriter, r *http.Request) {
			_, fromGithub := r.Header["X-Github-Event"]
			_, fromGitlab := r.Header["X-Gitlab-Event"]
			switch {
			case fromGithub:
				hook, _ := github.New(github.Options.Secret(t.Secret))
				payload, err := hook.Parse(r, github.PushEvent, github.PingEvent)
				if err != nil {
					log.Printf("github hook.Parse: %s\n", err)
					return
				}
				switch payload := payload.(type) {
				case github.PushPayload:
					log.Printf("path:%s server:%s channel:%s\n", t.Path, t.Server, t.Channel)
					pd := newPushDataFromGithub(&payload)
					for _, l := range pd.buildPushLines() {
						say(t, l)
						log.Printf("  %s\n", l)
					}
				case github.PingPayload:
					log.Printf("pinged: %+v\n", payload)
				}
			case fromGitlab:
				hook, _ := gitlab.New(gitlab.Options.Secret(t.Secret))
				payload, err := hook.Parse(r, gitlab.PushEvents)
				if err != nil {
					log.Printf("gitlab hook.Parse: %s\n", err)
					log.Printf("  %#v\n", r.Header)
					return
				}
				switch payload := payload.(type) {
				case gitlab.PushEventPayload:
					log.Printf("path:%s channel:%s\n", t.Path, t.Channel)
					pd := newPushDataFromGitlab(&payload)
					for _, l := range pd.buildPushLines() {
						say(t, l)
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

	log.Printf("githook listening on %s\n", gh.ListenAddr)
	err := http.ListenAndServe(gh.ListenAddr, nil)
	if err != nil {
		// TODO be more graceful perhaps. quit at all?
		quit(err.Error())
		log.Fatal(err)
	}
}

// TODO an interface that can be implemented for github/gitlab/etc?
type pushData struct {
	repo     string
	repoFull string
	pusher   string
	verb     string
	count    int64
	branch   string
	commits  []commitData
}
type commitData struct {
	message   string
	id        string
	committer string
}

func newPushDataFromGithub(push *github.PushPayload) *pushData {
	pd := pushData{}
	pd.repo = push.Repository.Name
	pd.repoFull = push.Repository.FullName
	pd.pusher = push.Pusher.Name
	pd.verb = "pushed"
	if push.Forced {
		pd.verb = "force-" + pd.verb
	}
	pd.count = int64(len(push.Commits))
	pd.branch = util.Last(strings.Split(push.Ref, "/"))
	for i := range push.Commits {
		pd.commits = append(pd.commits,
			commitData{
				message:   push.Commits[i].Message,
				id:        push.Commits[i].ID,
				committer: push.Commits[i].Committer.Name,
			})
	}
	return &pd
}

func newPushDataFromGitlab(push *gitlab.PushEventPayload) *pushData {
	pd := pushData{}
	pd.repo = push.Project.Name
	pd.repoFull = push.Project.PathWithNamespace
	pd.pusher = push.UserName
	pd.verb = "pushed"
	// TODO no force-pushed?
	pd.count = push.TotalCommitsCount
	pd.branch = util.Last(strings.Split(push.Ref, "/"))
	for i := range push.Commits {
		pd.commits = append(pd.commits,
			commitData{
				message:   push.Commits[i].Message,
				id:        push.Commits[i].ID,
				committer: push.Commits[i].Author.Name,
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
	l := fmt.Sprintf("[%s] %s %s %d %s to %s:",
		pd.repo, pd.pusher, pd.verb, pd.count, noun, pd.branch)
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
