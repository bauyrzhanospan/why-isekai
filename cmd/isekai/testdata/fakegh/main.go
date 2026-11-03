// fakegh imitates the few gh commands isekai uses, keeping issues in the
// JSON file named by FAKE_GH_DB. Test-only.
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
)

type comment struct {
	Author struct {
		Login string `json:"login"`
	} `json:"author"`
	Body      string `json:"body"`
	CreatedAt string `json:"createdAt"`
}

type issue struct {
	Number   int       `json:"number"`
	Title    string    `json:"title"`
	Body     string    `json:"body"`
	State    string    `json:"state"`
	URL      string    `json:"url"`
	Labels   []string  `json:"labels"`
	Comments []comment `json:"comments"`
}

func load() []issue {
	var is []issue
	data, _ := os.ReadFile(os.Getenv("FAKE_GH_DB"))
	json.Unmarshal(data, &is)
	return is
}

func save(is []issue) {
	data, _ := json.MarshalIndent(is, "", " ")
	os.WriteFile(os.Getenv("FAKE_GH_DB"), data, 0o644)
}

func flag(args []string, name string) []string {
	var v []string
	for i := 0; i < len(args)-1; i++ {
		if args[i] == name {
			v = append(v, args[i+1])
		}
	}
	return v
}

func main() {
	a := os.Args[1:]
	if len(a) < 2 {
		os.Exit(2)
	}
	is := load()
	switch a[0] + " " + a[1] {
	case "label create":
		return
	case "issue list":
		label := flag(a, "--label")[0]
		out := []issue{}
		for _, i := range is {
			for _, l := range i.Labels {
				if l == label {
					out = append(out, i)
				}
			}
		}
		json.NewEncoder(os.Stdout).Encode(out)
	case "issue create":
		n := len(is) + 1
		i := issue{Number: n, Title: flag(a, "--title")[0], Body: flag(a, "--body")[0], State: "OPEN", Labels: flag(a, "--label"), Comments: []comment{}}
		i.URL = "https://github.com/o/r/issues/" + strconv.Itoa(n)
		save(append(is, i))
		fmt.Println(i.URL)
	case "issue close", "issue comment":
		n, _ := strconv.Atoi(a[2])
		for k := range is {
			if is[k].Number == n {
				if a[1] == "close" {
					is[k].State = "CLOSED"
				}
				body := flag(a, "--comment")
				if a[1] == "comment" {
					body = flag(a, "--body")
				}
				if len(body) > 0 {
					c := comment{Body: body[0], CreatedAt: "2026-11-03T10:00:00Z"}
					c.Author.Login = os.Getenv("FAKE_GH_USER")
					is[k].Comments = append(is[k].Comments, c)
				}
			}
		}
		save(is)
	default:
		fmt.Fprintln(os.Stderr, "fakegh: unsupported", a)
		os.Exit(1)
	}
}
