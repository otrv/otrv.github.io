package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html/template"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"os"
	"path/filepath"
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
	"golang.org/x/image/font"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
)

const (
	dateLayout        = "2006-01-02"
	dateDisplayLayout = "Jan 02, 2006"
	siteURL           = "https://otrv.dev"
	gaID              = "G-DZ4KVNJVCR"
)

var (
	md = goldmark.New(
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

	postTmpl  = template.Must(template.ParseFiles("templates/post.gohtml"))
	indexTmpl = template.Must(template.ParseFiles("templates/index.gohtml"))
	feedTmpl  = texttemplate.Must(texttemplate.New("feed.xml").Funcs(texttemplate.FuncMap{
		"escape": func(s string) string {
			var buf bytes.Buffer
			template.HTMLEscape(&buf, []byte(s))
			return buf.String()
		},
		"cdata": func(s any) string {
			str := fmt.Sprintf("%v", s)
			return strings.ReplaceAll(str, "]]>", "]]]]><![CDATA[>")
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
	JSONLD      template.JS
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

type IndexData struct {
	Posts []Post
	GAID  string
}

type PostData struct {
	Post
	GAID string
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

	if err := generateSocialImage("social-index.png", "Özgür Tanrıverdi", "Software engineer in Istanbul."); err != nil {
		panic(err)
	}
	for _, post := range posts {
		if err := generateSocialImage(post.SocialImage(), post.Title, "Özgür Tanrıverdi / "+post.DateString()); err != nil {
			panic(err)
		}
	}

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

		path := filepath.Join(dir, entry.Name())
		content, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("read post %s: %w", path, err)
		}

		post, err := parsePost(entry.Name(), content)
		if err != nil {
			return nil, fmt.Errorf("parse post %s: %w", entry.Name(), err)
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

	ld := jsonLD{
		Context:       "https://schema.org",
		Type:          "BlogPosting",
		Headline:      meta.Title,
		Description:   meta.Description,
		DatePublished: date.Format(time.RFC3339),
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
			ID:   siteURL + "/" + slug + ".html",
		},
	}
	if meta.Cover != "" {
		ld.Image = siteURL + "/" + meta.Cover
	}
	jsonLDBytes, _ := json.Marshal(ld)

	return Post{
		Title:       meta.Title,
		Date:        date,
		Description: meta.Description,
		Cover:       meta.Cover,
		Slug:        slug,
		Content:     template.HTML(buf.String()),
		JSONLD:      template.JS(jsonLDBytes),
	}, nil
}

func generatePostPages(posts []Post) error {
	for _, post := range posts {
		path := filepath.Join("public", post.Slug+".html")
		f, err := os.Create(path)
		if err != nil {
			return fmt.Errorf("create post page %s: %w", path, err)
		}

		if err := postTmpl.Execute(f, PostData{Post: post, GAID: gaID}); err != nil {
			f.Close()
			return fmt.Errorf("render post %s: %w", post.Slug, err)
		}
		f.Close()
	}

	return nil
}

func generateIndex(posts []Post) error {
	f, err := os.Create("public/index.html")
	if err != nil {
		return fmt.Errorf("create index: %w", err)
	}
	defer f.Close()

	if err := indexTmpl.Execute(f, IndexData{Posts: posts, GAID: gaID}); err != nil {
		return fmt.Errorf("render index: %w", err)
	}
	return nil
}

type FeedData struct {
	Updated string
	Posts   []Post
}

func generateFeed(posts []Post) error {
	f, err := os.Create("public/feed.xml")
	if err != nil {
		return fmt.Errorf("create feed: %w", err)
	}
	defer f.Close()

	var updated time.Time
	if len(posts) > 0 {
		updated = posts[0].Date
	} else {
		updated = time.Now()
	}

	if err := feedTmpl.ExecuteTemplate(f, "feed.xml", FeedData{
		Updated: updated.Format(time.RFC3339),
		Posts:   posts,
	}); err != nil {
		return fmt.Errorf("render feed: %w", err)
	}
	return nil
}

func generateSitemap(posts []Post) error {
	f, err := os.Create("public/sitemap.xml")
	if err != nil {
		return fmt.Errorf("create sitemap: %w", err)
	}
	defer f.Close()

	if err := sitemapTmpl.ExecuteTemplate(f, "sitemap.xml", IndexData{Posts: posts}); err != nil {
		return fmt.Errorf("render sitemap: %w", err)
	}
	return nil
}

func copyStaticFiles(srcDir, dstDir string) error {
	entries, err := os.ReadDir(srcDir)
	if err != nil {
		return fmt.Errorf("read static dir %s: %w", srcDir, err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		src := filepath.Join(srcDir, entry.Name())
		dst := filepath.Join(dstDir, entry.Name())
		content, err := os.ReadFile(src)
		if err != nil {
			return fmt.Errorf("read static file %s: %w", src, err)
		}
		if err := os.WriteFile(dst, content, 0o644); err != nil {
			return fmt.Errorf("write static file %s: %w", dst, err)
		}
	}
	return nil
}

// Social images are separate from article illustrations, which may be too small
// or the wrong aspect ratio for link previews. Keep text inside crop-safe margins.
func generateSocialImage(filename, title, subtitle string) error {
	data, err := os.ReadFile("static/ibm-plex-mono-regular.ttf")
	if err != nil {
		return err
	}
	typeface, err := opentype.Parse(data)
	if err != nil {
		return err
	}

	canvas := image.NewRGBA(image.Rect(0, 0, 1200, 630))
	draw.Draw(canvas, canvas.Bounds(), image.NewUniform(color.RGBA{0x1c, 0x1b, 0x19, 0xff}), image.Point{}, draw.Src)
	ink := image.NewUniform(color.RGBA{0xe8, 0xe3, 0xda, 0xff})
	sand := image.NewUniform(color.RGBA{0xc7, 0xb5, 0x8d, 0xff})
	muted := image.NewUniform(color.RGBA{0xa6, 0xa0, 0x96, 0xff})

	// Wrap at words and reduce the size for longer titles rather than cropping them.
	var titleFace font.Face
	var lines []string
	var lineHeight int
	for size := 56.0; size >= 20; size -= 2 {
		face, err := opentype.NewFace(typeface, &opentype.FaceOptions{Size: size, DPI: 72})
		if err != nil {
			return err
		}
		lines = nil
		line := ""
		fits := true
		for _, word := range strings.Fields(title) {
			if font.MeasureString(face, word).Ceil() > 1040 {
				fits = false
				break
			}
			candidate := strings.TrimSpace(line + " " + word)
			if font.MeasureString(face, candidate).Ceil() > 1040 {
				lines = append(lines, line)
				line = word
			} else {
				line = candidate
			}
		}
		lines = append(lines, line)
		if fits && len(lines) <= 3 {
			titleFace = face
			lineHeight = int(size * 1.3)
			break
		}
		face.Close()
	}
	if titleFace == nil {
		return fmt.Errorf("title is too long for social image: %s", title)
	}
	defer titleFace.Close()
	// Balance two-line titles so a final word does not sit alone.
	if len(lines) == 2 {
		words := strings.Fields(title)
		bestWidth := 1041
		for i := 1; i < len(words); i++ {
			left, right := strings.Join(words[:i], " "), strings.Join(words[i:], " ")
			width := max(font.MeasureString(titleFace, left).Ceil(), font.MeasureString(titleFace, right).Ceil())
			if width < bestWidth {
				lines = []string{left, right}
				bestWidth = width
			}
		}
	}
	labelFace, err := opentype.NewFace(typeface, &opentype.FaceOptions{Size: 26, DPI: 72})
	if err != nil {
		return err
	}
	defer labelFace.Close()
	drawer := font.Drawer{Dst: canvas, Src: sand, Face: labelFace, Dot: fixed.P(80, 100)}
	drawer.DrawString("otrv.dev")
	drawer.Face = titleFace
	drawer.Src = ink
	for i, line := range lines {
		drawer.Dot = fixed.P(80, 330-(len(lines)-1)*lineHeight/2+i*lineHeight)
		drawer.DrawString(line)
	}
	drawer.Face = labelFace
	drawer.Src = muted
	drawer.Dot = fixed.P(80, 530)
	drawer.DrawString(subtitle)

	f, err := os.Create(filepath.Join("public", filename))
	if err != nil {
		return err
	}
	if err := png.Encode(f, canvas); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}
