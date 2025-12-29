package main

import (
	"bytes"
	"fmt"
	"html/template"
	"os"
	"path/filepath"
	"sort"
	"strings"
	texttemplate "text/template"
	"time"

	"github.com/yuin/goldmark"
	highlighting "github.com/yuin/goldmark-highlighting/v2"
)

const (
	dateLayout        = "2006-01-02"
	dateDisplayLayout = "Jan 2, 2006"
	siteURL           = "https://otrv.github.io"
)

var (
	md = goldmark.New(
		goldmark.WithExtensions(
			highlighting.NewHighlighting(
				highlighting.WithStyle("vim"),
			),
		),
	)

	postTmpl  = template.Must(template.ParseFiles("templates/post.gohtml"))
	indexTmpl = template.Must(template.ParseFiles("templates/index.gohtml"))
	feedTmpl = texttemplate.Must(texttemplate.New("feed.xml").Funcs(texttemplate.FuncMap{
		"escape": func(s string) string {
			var buf bytes.Buffer
			template.HTMLEscape(&buf, []byte(s))
			return buf.String()
		},
	}).ParseFiles("templates/feed.xml"))
)

type Post struct {
	Title       string
	Date        time.Time
	Description string
	Slug        string
	Content     template.HTML
}

func (p Post) DateString() string {
	return p.Date.Format(dateDisplayLayout)
}

func (p Post) DateISO() string {
	return p.Date.Format(dateLayout)
}

func (p Post) DateRFC3339() string {
	return p.Date.Format(time.RFC3339)
}

type IndexData struct {
	Posts []Post
}

func main() {
	if err := os.MkdirAll("public", 0o755); err != nil {
		panic(err)
	}

	posts, err := parsePosts("posts")
	if err != nil {
		panic(err)
	}

	sort.Slice(posts, func(i, j int) bool {
		return posts[i].Date.After(posts[j].Date)
	})

	if err := generatePostPages(posts); err != nil {
		panic(err)
	}

	if err := generateIndex(posts); err != nil {
		panic(err)
	}

	if err := generateFeed(posts); err != nil {
		panic(err)
	}
}

func parsePosts(dir string) ([]Post, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	var posts []Post
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}

		content, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			return nil, err
		}

		post, err := parsePost(entry.Name(), content)
		if err != nil {
			return nil, err
		}

		posts = append(posts, post)
	}

	return posts, nil
}

func parsePost(filename string, content []byte) (Post, error) {
	lines := strings.Split(string(content), "\n")

	var title, description string
	var date time.Time
	bodyStart := 0

	if len(lines) > 0 && strings.TrimSpace(lines[0]) == "---" {
		foundClosing := false
		for i := 1; i < len(lines); i++ {
			line := strings.TrimSpace(lines[i])
			if line == "---" {
				bodyStart = i + 1
				foundClosing = true
				break
			}
			if strings.HasPrefix(line, "title:") {
				title = strings.TrimSpace(strings.TrimPrefix(line, "title:"))
			}
			if strings.HasPrefix(line, "date:") {
				dateStr := strings.TrimSpace(strings.TrimPrefix(line, "date:"))
				parsed, err := time.Parse(dateLayout, dateStr)
				if err != nil {
					return Post{}, fmt.Errorf("invalid date %q in %s: %w", dateStr, filename, err)
				}
				date = parsed
			}
			if strings.HasPrefix(line, "description:") {
				description = strings.TrimSpace(strings.TrimPrefix(line, "description:"))
			}
		}
		if !foundClosing {
			return Post{}, fmt.Errorf("missing closing front matter --- in %s", filename)
		}
	}

	body := strings.Join(lines[bodyStart:], "\n")

	if title == "" {
		return Post{}, fmt.Errorf("missing title in %s", filename)
	}
	if date.IsZero() {
		return Post{}, fmt.Errorf("missing or invalid date in %s", filename)
	}

	var buf bytes.Buffer
	if err := md.Convert([]byte(body), &buf); err != nil {
		return Post{}, err
	}

	slug := strings.TrimSuffix(filename, ".md")

	return Post{
		Title:       title,
		Date:        date,
		Description: description,
		Slug:        slug,
		Content:     template.HTML(buf.String()),
	}, nil
}

func generatePostPages(posts []Post) error {
	for _, post := range posts {
		f, err := os.Create(filepath.Join("public", post.Slug+".html"))
		if err != nil {
			return err
		}

		if err := postTmpl.Execute(f, post); err != nil {
			f.Close()
			return err
		}
		f.Close()
	}

	return nil
}

func generateIndex(posts []Post) error {
	f, err := os.Create("public/index.html")
	if err != nil {
		return err
	}
	defer f.Close()

	return indexTmpl.Execute(f, IndexData{Posts: posts})
}

type FeedData struct {
	Updated string
	Posts   []Post
}

func generateFeed(posts []Post) error {
	f, err := os.Create("public/feed.xml")
	if err != nil {
		return err
	}
	defer f.Close()

	return feedTmpl.ExecuteTemplate(f, "feed.xml", FeedData{
		Updated: time.Now().Format(time.RFC3339),
		Posts:   posts,
	})
}
