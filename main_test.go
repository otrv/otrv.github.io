package main

import (
	"crypto/sha256"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"image/png"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"
)

func siteFixture(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	for _, name := range []string{"templates", "static"} {
		if err := os.CopyFS(filepath.Join(dir, name), os.DirFS(name)); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Mkdir(filepath.Join(dir, "posts"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Chdir(dir)
}

func writeFixture(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func writeSizedFixture(t *testing.T, path string, size int64) {
	t.Helper()
	writeFixture(t, path, "")
	if err := os.Truncate(path, size); err != nil {
		t.Fatal(err)
	}
}

func readFixture(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func siteSnapshot(t *testing.T) map[string][32]byte {
	t.Helper()
	files := make(map[string][32]byte)
	err := filepath.WalkDir("public", func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !entry.IsDir() {
			files[path] = sha256.Sum256([]byte(readFixture(t, path)))
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return files
}

func TestBuildSite(t *testing.T) {
	siteFixture(t)
	writeFixture(t, "static/images/cover.svg", `<svg xmlns="http://www.w3.org/2000/svg"/>`)
	writeFixture(t, "posts/z-old.md", "---\ntitle: Old\ndate: 2020-01-01\n---\nOld body")
	writeFixture(t, "posts/b-new.md", "---\ntitle: Second\ndate: 2026-03-17\n---\nSecond body")
	writeFixture(t, "posts/a-new.md", "---\ntitle: 'A & <B>'\ndate: 2026-03-17\ndescription: '<b>Text & ]]> literal</b>'\ncover: images/cover.svg\n---\n**Bold** and `]]>`\n\n```go\nfmt.Println(42)\n```\n")
	if err := buildSite(); err != nil {
		t.Fatal(err)
	}
	index := readFixture(t, "public/index.html")
	first, second, third := strings.Index(index, "/a-new.html"), strings.Index(index, "/b-new.html"), strings.Index(index, "/z-old.html")
	if first < 0 || second <= first || third <= second {
		t.Fatal("index must sort by descending date, then filename")
	}
	page := readFixture(t, "public/a-new.html")
	for _, want := range []string{"A &amp; &lt;B&gt;", "<strong>Bold</strong>", "<code>", "Println", `src="/images/cover.svg"`} {
		if !strings.Contains(page, want) {
			t.Errorf("page missing %q", want)
		}
	}
	jsonScript := regexp.MustCompile(`(?s)<script type="application/ld\+json">(.*?)</script>`).FindStringSubmatch(page)
	if len(jsonScript) != 2 {
		t.Fatal("missing structured data")
	}
	var structured struct {
		Headline      string `json:"headline"`
		DatePublished string `json:"datePublished"`
		Image         string `json:"image"`
	}
	if err := json.Unmarshal([]byte(jsonScript[1]), &structured); err != nil {
		t.Fatal(err)
	}
	if structured.Headline != "A & <B>" || structured.DatePublished != "2026-03-17T00:00:00Z" || structured.Image != "https://otrv.dev/images/cover.svg" {
		t.Fatalf("incorrect structured data: %+v", structured)
	}
	var feed struct {
		Updated string `xml:"updated"`
		Entries []struct {
			Title   string `xml:"title"`
			Summary struct {
				Type string `xml:"type,attr"`
				Text string `xml:",chardata"`
			} `xml:"summary"`
			Content string `xml:"content"`
		} `xml:"entry"`
	}
	if err := xml.Unmarshal([]byte(readFixture(t, "public/feed.xml")), &feed); err != nil {
		t.Fatal(err)
	}
	if feed.Updated != "2026-03-17T00:00:00Z" || len(feed.Entries) != 3 {
		t.Fatalf("incorrect feed: %+v", feed)
	}
	entry := feed.Entries[0]
	if entry.Title != "A & <B>" || entry.Summary.Type != "text" || entry.Summary.Text != "<b>Text & ]]> literal</b>" {
		t.Fatalf("feed metadata must retain literal text: %+v", entry)
	}
	if !strings.Contains(entry.Content, "<strong>Bold</strong>") || !strings.Contains(entry.Content, "]]&gt;") {
		t.Fatalf("incorrect feed content: %s", entry.Content)
	}
	var sitemap struct {
		Locations []string `xml:"url>loc"`
	}
	if err := xml.Unmarshal([]byte(readFixture(t, "public/sitemap.xml")), &sitemap); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(sitemap.Locations, []string{"https://otrv.dev/", "https://otrv.dev/a-new.html", "https://otrv.dev/b-new.html", "https://otrv.dev/z-old.html"}) {
		t.Fatalf("incorrect sitemap: %v", sitemap.Locations)
	}
	if readFixture(t, "public/images/cover.svg") != readFixture(t, "static/images/cover.svg") {
		t.Fatal("nested static file changed")
	}
	for _, name := range []string{"index", "a-new", "b-new", "z-old"} {
		f, err := os.Open("public/social-" + name + ".png")
		if err != nil {
			t.Fatal(err)
		}
		config, decodeErr := png.DecodeConfig(f)
		if err := errors.Join(decodeErr, f.Close()); err != nil {
			t.Fatal(err)
		}
		if config.Width != 1200 || config.Height != 630 {
			t.Fatalf("incorrect social image size: %+v", config)
		}
	}
	before := siteSnapshot(t)
	if err := buildSite(); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(before, siteSnapshot(t)) {
		t.Fatal("identical inputs must produce identical output")
	}
	if err := os.Remove("posts/z-old.md"); err != nil {
		t.Fatal(err)
	}
	if err := buildSite(); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"public/z-old.html", "public/social-z-old.png"} {
		if _, err := os.Stat(path); !errors.Is(err, fs.ErrNotExist) {
			t.Fatalf("removed post still has output at %s: %v", path, err)
		}
	}
}

func TestBuildFailurePreservesSite(t *testing.T) {
	validPost := "---\ntitle: Valid\ndate: 2026-03-17\n---\nBody"
	for _, test := range []struct {
		name    string
		path    string
		content string
		want    string
	}{
		{"front matter", "posts/valid.md", "Body", "missing front matter"},
		{"malformed YAML", "posts/valid.md", "---\ntitle: [\n---\n", "invalid front matter"},
		{"blank title", "posts/valid.md", "---\ntitle: '  '\ndate: 2026-03-17\n---\n", "missing title"},
		{"date", "posts/valid.md", strings.Replace(validPost, "2026-03-17", "2026-02-30", 1), "invalid date"},
		{"filename", "posts/a&b.md", validPost, "invalid post filename"},
		{"reserved output", "posts/index.md", validPost, "file exists"},
		{"static collision", "static/feed.xml", "overwritten feed", "file exists"},
		{"missing cover", "posts/valid.md", strings.Replace(validPost, "title: Valid", "title: Valid\ncover: missing.png", 1), "read cover"},
		{"cover traversal", "posts/valid.md", strings.Replace(validPost, "title: Valid", "title: Valid\ncover: ../outside.png", 1), "invalid cover path"},
		{"template parsing", "templates/post.gohtml", "{{", "read HTML templates"},
		{"template rendering", "templates/post.gohtml", "{{.Unknown}}", "write output"},
		{"font", "static/ibm-plex-mono-regular.ttf", "not a font", "parse social image font"},
		{"title overflow", "posts/valid.md", strings.Replace(validPost, "title: Valid", "title: "+strings.Repeat("W", 200), 1), "title is too long"},
		{"size", "posts/valid.md", validPost + strings.Repeat("x", 1<<20), "exceeds 1048576 bytes"},
	} {
		t.Run(test.name, func(t *testing.T) {
			siteFixture(t)
			writeFixture(t, "posts/valid.md", validPost)
			if err := buildSite(); err != nil {
				t.Fatal(err)
			}
			before := siteSnapshot(t)
			writeFixture(t, test.path, test.content)
			if err := buildSite(); err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("want error containing %q, got %v", test.want, err)
			}
			if !reflect.DeepEqual(before, siteSnapshot(t)) {
				t.Fatal("failed build changed the previous site")
			}
		})
	}
}

func TestBuildEmptySite(t *testing.T) {
	siteFixture(t)
	if err := buildSite(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(readFixture(t, "public/feed.xml"), "<updated>1970-01-01T00:00:00Z</updated>") {
		t.Fatal("empty feed must use a deterministic timestamp")
	}
	before := siteSnapshot(t)
	if err := buildSite(); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(before, siteSnapshot(t)) {
		t.Fatal("empty site output changed")
	}
}

func TestBuildRejectsStaticSymlinks(t *testing.T) {
	for _, kind := range []string{"root", "file", "directory"} {
		t.Run(kind, func(t *testing.T) {
			siteFixture(t)
			if err := buildSite(); err != nil {
				t.Fatal(err)
			}
			before := siteSnapshot(t)
			external := t.TempDir()
			sentinel := filepath.Join(external, "private.txt")
			writeFixture(t, sentinel, "must not be published")
			target, link := external, "static/shared"
			switch kind {
			case "root":
				target, link = filepath.Join(external, "assets"), "static"
				if err := os.Rename("static", target); err != nil {
					t.Fatal(err)
				}
			case "file":
				target, link = sentinel, "static/private.txt"
			}
			if err := os.Symlink(target, link); err != nil {
				t.Fatal(err)
			}
			if err := buildSite(); err == nil || !strings.Contains(err.Error(), "unsupported static entry") {
				t.Fatalf("want static symlink rejection, got %v", err)
			}
			if !reflect.DeepEqual(before, siteSnapshot(t)) {
				t.Fatal("symlink rejection changed the previous site")
			}
			if readFixture(t, sentinel) != "must not be published" {
				t.Fatal("external sentinel changed")
			}
		})
	}
}

func TestBuildRejectsGeneratedPageCover(t *testing.T) {
	for _, date := range []string{"2026-03-16", "2026-03-18"} {
		t.Run(date, func(t *testing.T) {
			siteFixture(t)
			writeFixture(t, "posts/first.md", "---\ntitle: First\ndate: 2026-03-17\n---\nBody")
			if err := buildSite(); err != nil {
				t.Fatal(err)
			}
			before := siteSnapshot(t)
			writeFixture(t, "posts/second.md", "---\ntitle: Second\ndate: "+date+"\ncover: first.html\n---\nBody")
			if err := buildSite(); err == nil || !strings.Contains(err.Error(), "read cover for second") {
				t.Fatalf("want missing static cover rejection, got %v", err)
			}
			if !reflect.DeepEqual(before, siteSnapshot(t)) {
				t.Fatal("cover rejection changed the previous site")
			}
		})
	}
}

func TestStaticLimits(t *testing.T) {
	t.Run("file bytes", func(t *testing.T) {
		t.Chdir(t.TempDir())
		writeSizedFixture(t, "static/asset", 16<<20)
		if err := copyStaticFiles("static", "accepted"); err != nil {
			t.Fatal(err)
		}
		if info, err := os.Stat("accepted/asset"); err != nil || info.Size() != 16<<20 {
			t.Fatalf("file at byte limit was not copied in full: %v", err)
		}
		writeSizedFixture(t, "static/asset", (16<<20)+1)
		if err := copyStaticFiles("static", "rejected"); err == nil || !strings.Contains(err.Error(), "available bytes") {
			t.Fatalf("want file byte limit rejection, got %v", err)
		}
	})
	t.Run("total bytes", func(t *testing.T) {
		t.Chdir(t.TempDir())
		for i := range 8 {
			writeSizedFixture(t, fmt.Sprintf("static/asset-%d", i), 16<<20)
		}
		writeFixture(t, "static/empty", "")
		if err := copyStaticFiles("static", "accepted"); err != nil {
			t.Fatalf("tree at total byte limit must be accepted: %v", err)
		}
		if _, err := os.Stat("accepted/empty"); err != nil {
			t.Fatal(err)
		}
		writeFixture(t, "static/overflow", "x")
		if err := copyStaticFiles("static", "rejected"); err == nil || !strings.Contains(err.Error(), "available bytes") {
			t.Fatalf("want total byte limit rejection, got %v", err)
		}
	})
	t.Run("entries", func(t *testing.T) {
		t.Chdir(t.TempDir())
		for _, dir := range []string{"a", "b"} {
			for i := range 2047 {
				writeFixture(t, fmt.Sprintf("static/%s/entry-%04d", dir, i), "")
			}
		}
		if err := copyStaticFiles("static", "accepted"); err != nil {
			t.Fatalf("tree at entry limit must be accepted: %v", err)
		}
		writeFixture(t, "static/b/overflow", "")
		if err := copyStaticFiles("static", "rejected"); err == nil || !strings.Contains(err.Error(), "4096 entries") {
			t.Fatalf("want entry count rejection, got %v", err)
		}
	})
	t.Run("depth", func(t *testing.T) {
		t.Chdir(t.TempDir())
		path := filepath.Join("static", strings.Repeat("nested/", 8))
		writeFixture(t, filepath.Join(path, "asset"), "deep asset")
		if err := copyStaticFiles("static", "accepted"); err != nil {
			t.Fatalf("tree at depth limit must be accepted: %v", err)
		}
		if readFixture(t, filepath.Join("accepted", strings.Repeat("nested/", 8), "asset")) != "deep asset" {
			t.Fatal("deep asset changed")
		}
		writeFixture(t, filepath.Join(path, "overflow", "asset"), "too deep")
		if err := copyStaticFiles("static", "rejected"); err == nil || !strings.Contains(err.Error(), "8 nested directory levels") {
			t.Fatalf("want directory depth rejection, got %v", err)
		}
	})
}

func TestBuildFontLimit(t *testing.T) {
	siteFixture(t)
	path := "static/ibm-plex-mono-regular.ttf"
	font := readFixture(t, path)
	font += strings.Repeat("\x00", (8<<20)-len(font))
	writeFixture(t, path, font)
	if err := buildSite(); err != nil {
		t.Fatalf("valid font at byte limit must be accepted: %v", err)
	}
	before := siteSnapshot(t)
	writeFixture(t, path, font+"\x00")
	if err := buildSite(); err == nil || !strings.Contains(err.Error(), "exceeds 8388608 bytes") {
		t.Fatalf("want font byte limit rejection, got %v", err)
	}
	if !reflect.DeepEqual(before, siteSnapshot(t)) {
		t.Fatal("font limit rejection changed the previous site")
	}
}

func TestPostLimits(t *testing.T) {
	t.Run("bytes", func(t *testing.T) {
		t.Chdir(t.TempDir())
		content := "---\ntitle: Valid\ndate: 2026-03-17\n---\n"
		content += strings.Repeat("x", (1<<20)-len(content))
		writeFixture(t, "posts/valid.md", content)
		if posts, err := readPosts("posts"); err != nil || len(posts) != 1 {
			t.Fatalf("post at byte limit must be accepted: %v", err)
		}
		writeFixture(t, "posts/valid.md", content+"x")
		if _, err := readPosts("posts"); err == nil {
			t.Fatal("post above byte limit must be rejected")
		}
	})
	t.Run("count", func(t *testing.T) {
		t.Chdir(t.TempDir())
		for i := range 256 {
			writeFixture(t, fmt.Sprintf("posts/post-%03d.md", i), "---\ntitle: Valid\ndate: 2026-03-17\n---\nBody")
		}
		if posts, err := readPosts("posts"); err != nil || len(posts) != 256 {
			t.Fatalf("post count at limit must be accepted: %v", err)
		}
		writeFixture(t, "posts/overflow.md", "---\ntitle: Valid\ndate: 2026-03-17\n---\nBody")
		if _, err := readPosts("posts"); err == nil {
			t.Fatal("post count above limit must be rejected")
		}
	})
	t.Run("directory entries", func(t *testing.T) {
		t.Chdir(t.TempDir())
		for i := range 1024 {
			writeFixture(t, fmt.Sprintf("posts/entry-%04d.txt", i), "ignored")
		}
		if posts, err := readPosts("posts"); err != nil || len(posts) != 0 {
			t.Fatalf("directory at entry limit must be accepted: %v", err)
		}
		writeFixture(t, "posts/overflow.txt", "ignored")
		if _, err := readPosts("posts"); err == nil {
			t.Fatal("directory above entry limit must be rejected")
		}
	})
}
