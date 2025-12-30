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
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
	"go.abhg.dev/goldmark/frontmatter"
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
			&frontmatter.Extender{},
		),
	)

	postTmpl  = template.Must(template.ParseFiles("templates/post.gohtml"))
	indexTmpl = template.Must(template.ParseFiles("templates/index.gohtml"))
	feedTmpl    = texttemplate.Must(texttemplate.New("feed.xml").Funcs(texttemplate.FuncMap{
		"escape": func(s string) string {
			var buf bytes.Buffer
			template.HTMLEscape(&buf, []byte(s))
			return buf.String()
		},
	}).ParseFiles("templates/feed.xml"))
	sitemapTmpl = texttemplate.Must(texttemplate.ParseFiles("templates/sitemap.xml"))
)

type Post struct {
	Title       string
	Date        time.Time
	Description string
	Cover       string
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

	if err := generateSitemap(posts); err != nil {
		panic(err)
	}

	if err := copyStaticFiles("static", "public"); err != nil {
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

type postMeta struct {
	Title       string `yaml:"title"`
	Date        string `yaml:"date"`
	Description string `yaml:"description"`
	Cover       string `yaml:"cover"`
}

func parsePost(filename string, content []byte) (Post, error) {
	ctx := parser.NewContext()
	doc := md.Parser().Parse(text.NewReader(content), parser.WithContext(ctx))

	d := frontmatter.Get(ctx)
	if d == nil {
		return Post{}, fmt.Errorf("missing front matter in %s", filename)
	}

	var meta postMeta
	if err := d.Decode(&meta); err != nil {
		return Post{}, fmt.Errorf("invalid front matter in %s: %w", filename, err)
	}

	if meta.Title == "" {
		return Post{}, fmt.Errorf("missing title in %s", filename)
	}

	date, err := time.Parse(dateLayout, meta.Date)
	if err != nil {
		return Post{}, fmt.Errorf("invalid date %q in %s: %w", meta.Date, filename, err)
	}

	var buf bytes.Buffer
	if err := md.Renderer().Render(&buf, content, doc); err != nil {
		return Post{}, err
	}

	slug := strings.TrimSuffix(filename, ".md")

	return Post{
		Title:       meta.Title,
		Date:        date,
		Description: meta.Description,
		Cover:       meta.Cover,
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

func generateSitemap(posts []Post) error {
	f, err := os.Create("public/sitemap.xml")
	if err != nil {
		return err
	}
	defer f.Close()

	return sitemapTmpl.ExecuteTemplate(f, "sitemap.xml", IndexData{Posts: posts})
}

func copyStaticFiles(srcDir, dstDir string) error {
	entries, err := os.ReadDir(srcDir)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		src := filepath.Join(srcDir, entry.Name())
		dst := filepath.Join(dstDir, entry.Name())
		content, err := os.ReadFile(src)
		if err != nil {
			return err
		}
		if err := os.WriteFile(dst, content, 0o644); err != nil {
			return err
		}
	}
	return nil
}
