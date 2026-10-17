package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"io"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	texttemplate "text/template"
	"time"

	"github.com/alecthomas/chroma/v2"
	"github.com/yuin/goldmark"
	highlighting "github.com/yuin/goldmark-highlighting/v2"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
	"go.abhg.dev/goldmark/frontmatter"
)

const (
	dateLayout         = "2006-01-02"
	dateDisplayLayout  = "Jan 02, 2006"
	siteURL            = "https://otrv.dev"
	gaID               = "G-DZ4KVNJVCR"
	postCountMax       = 256
	postBytesMax       = 1 << 20
	postEntriesMax     = 1024
	staticEntriesMax   = 4096
	staticDepthMax     = 8
	staticFileBytesMax = 16 << 20
	staticBytesMax     = 128 << 20
)

type Post struct {
	Title       string
	Date        time.Time
	Description string
	Cover       string
	Slug        string
	Content     template.HTML
}

type jsonLD struct {
	Context          string       `json:"@context"`
	Type             string       `json:"@type"`
	Headline         string       `json:"headline"`
	Description      string       `json:"description,omitempty"`
	DatePublished    string       `json:"datePublished"`
	Author           jsonLDPerson `json:"author"`
	Publisher        jsonLDPerson `json:"publisher"`
	MainEntityOfPage jsonLDPage   `json:"mainEntityOfPage"`
	Image            string       `json:"image,omitempty"`
}

type jsonLDPerson struct {
	Type string `json:"@type"`
	Name string `json:"name"`
	URL  string `json:"url,omitempty"`
}

type jsonLDPage struct {
	Type string `json:"@type"`
	ID   string `json:"@id"`
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

func (p Post) SocialImage() string {
	return "social-" + p.Slug + ".png"
}

func (p Post) JSONLD() (template.JS, error) {
	data := jsonLD{
		Context:       "https://schema.org",
		Type:          "BlogPosting",
		Headline:      p.Title,
		Description:   p.Description,
		DatePublished: p.DateRFC3339(),
		Author: jsonLDPerson{
			Type: "Person",
			Name: "Özgür Tanrıverdi",
			URL:  siteURL,
		},
		Publisher: jsonLDPerson{
			Type: "Person",
			Name: "Özgür Tanrıverdi",
		},
		MainEntityOfPage: jsonLDPage{
			Type: "WebPage",
			ID:   siteURL + "/" + p.Slug + ".html",
		},
	}
	if p.Cover != "" {
		data.Image = siteURL + "/" + p.Cover
	}
	encoded, err := json.Marshal(data)
	return template.JS(encoded), err
}

type siteData struct {
	Posts   []Post
	GAID    string
	Updated string
}

type postData struct {
	Post
	GAID string
}

func main() {
	if err := buildSite(); err != nil {
		log.New(os.Stderr, "", 0).Fatal(err)
	}
}

func buildSite() (err error) {
	posts, err := readPosts("posts")
	if err != nil {
		return err
	}
	htmlTemplates, err := template.ParseFiles("templates/post.gohtml", "templates/index.gohtml")
	if err != nil {
		return fmt.Errorf("read HTML templates: %w", err)
	}
	xmlTemplates, err := texttemplate.New("xml").Funcs(texttemplate.FuncMap{
		"escape": template.HTMLEscapeString,
		"cdata": func(s template.HTML) string {
			return strings.ReplaceAll(string(s), "]]>", "]]]]><![CDATA[>")
		},
	}).ParseFiles("templates/feed.xml", "templates/sitemap.xml")
	if err != nil {
		return fmt.Errorf("read XML templates: %w", err)
	}

	buildDir, err := os.MkdirTemp(".", ".site-build-")
	if err != nil {
		return fmt.Errorf("create build directory: %w", err)
	}
	cleanupBuild := true
	defer func() {
		if cleanupBuild {
			err = errors.Join(err, os.RemoveAll(buildDir))
		}
	}()
	outputDir := filepath.Join(buildDir, "public")
	if err := copyStaticFiles("static", outputDir); err != nil {
		return fmt.Errorf("copy static files: %w", err)
	}
	for _, post := range posts {
		if post.Cover != "" {
			info, err := os.Stat(filepath.Join(outputDir, filepath.FromSlash(post.Cover)))
			if err != nil {
				return fmt.Errorf("read cover for %s: %w", post.Slug, err)
			}
			if !info.Mode().IsRegular() {
				return fmt.Errorf("cover for %s is not a regular file", post.Slug)
			}
		}
	}
	for _, post := range posts {
		if err := writeOutput(filepath.Join(outputDir, post.Slug+".html"), func(w io.Writer) error {
			return htmlTemplates.ExecuteTemplate(w, "post.gohtml", postData{Post: post, GAID: gaID})
		}); err != nil {
			return err
		}
	}

	data := siteData{Posts: posts, GAID: gaID, Updated: time.Unix(0, 0).UTC().Format(time.RFC3339)}
	if len(posts) > 0 {
		data.Updated = posts[0].DateRFC3339()
	}
	if err := writeOutput(filepath.Join(outputDir, "index.html"), func(w io.Writer) error {
		return htmlTemplates.ExecuteTemplate(w, "index.gohtml", data)
	}); err != nil {
		return err
	}
	for _, name := range []string{"feed.xml", "sitemap.xml"} {
		if err := writeOutput(filepath.Join(outputDir, name), func(w io.Writer) error {
			return xmlTemplates.ExecuteTemplate(w, name, data)
		}); err != nil {
			return err
		}
	}
	if err := generateSocialImages(outputDir, posts); err != nil {
		return err
	}

	previousDir := filepath.Join(buildDir, "previous")
	renameErr := os.Rename("public", previousDir)
	if renameErr != nil && !errors.Is(renameErr, fs.ErrNotExist) {
		return fmt.Errorf("move previous site: %w", renameErr)
	}
	if err := os.Rename(outputDir, "public"); err != nil {
		if renameErr == nil {
			if restoreErr := os.Rename(previousDir, "public"); restoreErr != nil {
				cleanupBuild = false
				return fmt.Errorf("publish built site; previous site retained at %s: %w", previousDir, errors.Join(err, restoreErr))
			}
		}
		return fmt.Errorf("publish built site: %w", err)
	}
	return nil
}

func readPosts(dir string) ([]Post, error) {
	f, err := os.Open(dir)
	if err != nil {
		return nil, fmt.Errorf("read posts: %w", err)
	}
	entries, readErr := f.ReadDir(postEntriesMax + 1)
	if errors.Is(readErr, io.EOF) {
		readErr = nil
	}
	if err := errors.Join(readErr, f.Close()); err != nil {
		return nil, fmt.Errorf("read posts: %w", err)
	}
	if len(entries) > postEntriesMax {
		return nil, fmt.Errorf("posts directory exceeds %d entries", postEntriesMax)
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Name() < entries[j].Name()
	})
	markdown := goldmark.New(
		goldmark.WithExtensions(
			highlighting.NewHighlighting(
				highlighting.WithCustomStyle(chroma.MustNewStyle("charcoal", chroma.StyleEntries{
					chroma.Background:    "bg:#242320 #e8e3da",
					chroma.Comment:       "#a6a096",
					chroma.Keyword:       "#c7b58d",
					chroma.NameBuiltin:   "#c3b5d5",
					chroma.LiteralString: "#b7c9a8",
					chroma.LiteralNumber: "#d6b2a2",
				})),
			),
			&frontmatter.Extender{},
		),
	)
	slugPattern := regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)
	var posts []Post
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}
		if !entry.Type().IsRegular() {
			return nil, fmt.Errorf("post %s is not a regular file", entry.Name())
		}
		if len(posts) == postCountMax {
			return nil, fmt.Errorf("post count exceeds %d", postCountMax)
		}
		slug := strings.TrimSuffix(entry.Name(), ".md")
		if !slugPattern.MatchString(slug) {
			return nil, fmt.Errorf("invalid post filename %q: use lowercase letters, digits, and single hyphens", entry.Name())
		}
		path := filepath.Join(dir, entry.Name())
		content, err := readFileLimited(path, postBytesMax)
		if err != nil {
			return nil, err
		}
		post, err := parsePost(markdown, slug, content)
		if err != nil {
			return nil, fmt.Errorf("parse post %s: %w", path, err)
		}
		posts = append(posts, post)
	}
	sort.SliceStable(posts, func(i, j int) bool {
		return posts[i].Date.After(posts[j].Date)
	})
	return posts, nil
}

func readFileLimited(path string, bytesMax int64) (content []byte, err error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open file %s: %w", path, err)
	}
	defer func() {
		err = errors.Join(err, f.Close())
	}()
	content, err = io.ReadAll(io.LimitReader(f, bytesMax+1))
	if err != nil {
		return nil, fmt.Errorf("read file %s: %w", path, err)
	}
	if int64(len(content)) > bytesMax {
		return nil, fmt.Errorf("file %s exceeds %d bytes", path, bytesMax)
	}
	return content, nil
}

func copyStaticFiles(srcDir, dstDir string) (err error) {
	info, err := os.Lstat(srcDir)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("unsupported static entry %s: expected a directory", srcDir)
	}
	source, err := os.OpenRoot(srcDir)
	if err != nil {
		return err
	}
	defer func() {
		err = errors.Join(err, source.Close())
	}()
	if err := os.MkdirAll(dstDir, 0o755); err != nil {
		return err
	}
	directories := []string{"."}
	entriesTotal := 0
	var bytesTotal int64
	for i := 0; i < len(directories); i++ {
		dir, err := source.Open(directories[i])
		if err != nil {
			return err
		}
		entries, readErr := dir.ReadDir(staticEntriesMax - entriesTotal + 1)
		if errors.Is(readErr, io.EOF) {
			readErr = nil
		}
		if err := errors.Join(readErr, dir.Close()); err != nil {
			return err
		}
		entriesTotal += len(entries)
		if entriesTotal > staticEntriesMax {
			return fmt.Errorf("static tree exceeds %d entries", staticEntriesMax)
		}
		for _, entry := range entries {
			path := filepath.Join(directories[i], entry.Name())
			if entry.IsDir() {
				if strings.Count(path, string(filepath.Separator))+1 > staticDepthMax {
					return fmt.Errorf("static directory %s exceeds %d nested directory levels", path, staticDepthMax)
				}
				if err := os.Mkdir(filepath.Join(dstDir, path), 0o755); err != nil {
					return err
				}
				directories = append(directories, path)
				continue
			}
			if !entry.Type().IsRegular() {
				return fmt.Errorf("unsupported static entry %s: expected a regular file or directory", path)
			}
			var copied int64
			limit := min(int64(staticFileBytesMax), staticBytesMax-bytesTotal)
			if err := writeOutput(filepath.Join(dstDir, path), func(w io.Writer) (err error) {
				f, err := source.Open(path)
				if err != nil {
					return err
				}
				defer func() {
					err = errors.Join(err, f.Close())
				}()
				info, err := f.Stat()
				if err != nil {
					return err
				}
				if !info.Mode().IsRegular() {
					return fmt.Errorf("unsupported static entry %s: expected a regular file", path)
				}
				copied, err = io.Copy(w, io.LimitReader(f, limit+1))
				if err != nil {
					return err
				}
				if copied > limit {
					return fmt.Errorf("static file %s exceeds %d available bytes (file limit %d, tree limit %d)", path, limit, staticFileBytesMax, staticBytesMax)
				}
				return nil
			}); err != nil {
				return err
			}
			bytesTotal += copied
		}
	}
	return nil
}

type postMeta struct {
	Title       string `yaml:"title"`
	Date        string `yaml:"date"`
	Description string `yaml:"description"`
	Cover       string `yaml:"cover"`
}

func parsePost(markdown goldmark.Markdown, slug string, content []byte) (Post, error) {
	ctx := parser.NewContext()
	doc := markdown.Parser().Parse(text.NewReader(content), parser.WithContext(ctx))
	metadata := frontmatter.Get(ctx)
	if metadata == nil {
		return Post{}, errors.New("missing front matter")
	}
	var meta postMeta
	if err := metadata.Decode(&meta); err != nil {
		return Post{}, fmt.Errorf("invalid front matter: %w", err)
	}
	meta.Title = strings.TrimSpace(meta.Title)
	if meta.Title == "" {
		return Post{}, errors.New("missing title")
	}
	date, err := time.Parse(dateLayout, meta.Date)
	if err != nil {
		return Post{}, fmt.Errorf("invalid date %q: %w", meta.Date, err)
	}
	if meta.Cover != "" && (!fs.ValidPath(meta.Cover) || strings.ContainsAny(meta.Cover, `\?#%`)) {
		return Post{}, fmt.Errorf("invalid cover path %q: use a relative path within static", meta.Cover)
	}
	var buf bytes.Buffer
	if err := markdown.Renderer().Render(&buf, content, doc); err != nil {
		return Post{}, fmt.Errorf("render Markdown: %w", err)
	}
	return Post{
		Title:       meta.Title,
		Date:        date,
		Description: meta.Description,
		Cover:       meta.Cover,
		Slug:        slug,
		Content:     template.HTML(buf.String()),
	}, nil
}

func writeOutput(path string, render func(io.Writer) error) error {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return fmt.Errorf("create output %s: %w", path, err)
	}
	if err := errors.Join(render(f), f.Close()); err != nil {
		return fmt.Errorf("write output %s: %w", path, err)
	}
	return nil
}
