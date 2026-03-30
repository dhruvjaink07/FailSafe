package main

import (
	"bytes"
	"flag"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer/html"
)

type docPage struct {
	Source   string
	Output   string
	NavLabel string
	Title    string
}

var apiPages = []docPage{
	{Source: "README.md", Output: "index.html", NavLabel: "Overview", Title: "FailSafe API Reference"},
	{Source: "backend-api.md", Output: "backend-api.html", NavLabel: "Backend API Contract", Title: "Backend API Contract"},
	{Source: "frontend-testing.md", Output: "frontend-testing.html", NavLabel: "Frontend Integration", Title: "Frontend Testing Integration Guide"},
	{Source: "postman-testing.md", Output: "postman-testing.html", NavLabel: "Postman Testing Guide", Title: "Postman Testing Guide"},
}

type templateData struct {
	Title    string
	Subtitle string
	Active   string
	Nav      []docPage
	BodyHTML template.HTML
}

const pageTemplate = `<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8" />
  <meta name="viewport" content="width=device-width, initial-scale=1" />
  <title>{{.Title}}</title>
  <link rel="stylesheet" href="styles.css" />
</head>
<body>
  <header>
    <h1>{{.Title}}</h1>
    <p>{{.Subtitle}}</p>
  </header>

  <div class="layout">
    <nav>
      <h2>Pages</h2>
      {{range .Nav}}
      <a href="{{.Output}}" {{if eq $.Active .Output}}class="active"{{end}}>{{.NavLabel}}</a>
      {{end}}
    </nav>

    <main>
      {{.BodyHTML}}
    </main>
  </div>

  <footer>
    Generated from docs/api markdown. API behavior only.
  </footer>

	<script>
		(function () {
			function fallbackCopy(text) {
				var ta = document.createElement('textarea');
				ta.value = text;
				ta.setAttribute('readonly', '');
				ta.style.position = 'absolute';
				ta.style.left = '-9999px';
				document.body.appendChild(ta);
				ta.select();
				document.execCommand('copy');
				document.body.removeChild(ta);
			}

			function copyText(text) {
				if (navigator.clipboard && window.isSecureContext) {
					return navigator.clipboard.writeText(text);
				}
				return new Promise(function (resolve) {
					fallbackCopy(text);
					resolve();
				});
			}

			var blocks = document.querySelectorAll('pre, .code');
			blocks.forEach(function (block) {
				var wrap = document.createElement('div');
				wrap.className = 'copy-wrap';
				block.parentNode.insertBefore(wrap, block);
				wrap.appendChild(block);

				var btn = document.createElement('button');
				btn.className = 'copy-btn';
				btn.type = 'button';
				btn.textContent = 'Copy';
				btn.addEventListener('click', function () {
					var text = block.innerText || block.textContent || '';
					copyText(text).then(function () {
						var old = btn.textContent;
						btn.textContent = 'Copied';
						setTimeout(function () { btn.textContent = old; }, 1200);
					});
				});
				wrap.appendChild(btn);
			});
		})();
	</script>
</body>
</html>`

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}

	switch os.Args[1] {
	case "build":
		if err := runBuild(); err != nil {
			log.Fatal(err)
		}
	case "serve":
		serveCmd := flag.NewFlagSet("serve", flag.ExitOnError)
		port := serveCmd.Int("port", 8090, "docs server port")
		_ = serveCmd.Parse(os.Args[2:])
		if err := runBuild(); err != nil {
			log.Fatal(err)
		}
		if err := runServe(*port); err != nil {
			log.Fatal(err)
		}
	default:
		usage()
		os.Exit(2)
	}
}

func usage() {
	fmt.Println("usage: go run ./cmd/docs [build|serve] [-port 8090]")
}

func runBuild() error {
	apiDir := filepath.Join("docs", "api")
	siteDir := filepath.Join("docs", "site")
	if err := os.MkdirAll(siteDir, 0o755); err != nil {
		return fmt.Errorf("create site dir: %w", err)
	}

	renderer := goldmark.New(
		goldmark.WithExtensions(extension.GFM),
		goldmark.WithParserOptions(parser.WithAutoHeadingID()),
		goldmark.WithRendererOptions(html.WithUnsafe()),
	)

	tpl, err := template.New("page").Parse(pageTemplate)
	if err != nil {
		return fmt.Errorf("parse html template: %w", err)
	}

	for _, pg := range apiPages {
		sourcePath := filepath.Join(apiDir, pg.Source)
		in, err := os.ReadFile(sourcePath)
		if err != nil {
			return fmt.Errorf("read %s: %w", sourcePath, err)
		}

		content := rewriteMarkdownLinks(string(in))
		var mdHTML bytes.Buffer
		if err := renderer.Convert([]byte(content), &mdHTML); err != nil {
			return fmt.Errorf("convert markdown %s: %w", sourcePath, err)
		}

		data := templateData{
			Title:    pg.Title,
			Subtitle: "FailSafe controller API documentation",
			Active:   pg.Output,
			Nav:      apiPages,
			BodyHTML: template.HTML(mdHTML.String()),
		}

		outPath := filepath.Join(siteDir, pg.Output)
		outFile, err := os.Create(outPath)
		if err != nil {
			return fmt.Errorf("create %s: %w", outPath, err)
		}

		if err := tpl.Execute(outFile, data); err != nil {
			_ = outFile.Close()
			return fmt.Errorf("render %s: %w", outPath, err)
		}
		if err := outFile.Close(); err != nil {
			return fmt.Errorf("close %s: %w", outPath, err)
		}

		fmt.Printf("generated %s from %s\n", outPath, sourcePath)
	}

	return ensureStyleSheet(siteDir)
}

func rewriteMarkdownLinks(content string) string {
	// Keep links local when moving from markdown to static html pages.
	re := regexp.MustCompile(`\(([^)]+)\.md\)`)
	return re.ReplaceAllStringFunc(content, func(match string) string {
		m := re.FindStringSubmatch(match)
		if len(m) != 2 {
			return match
		}
		base := m[1]
		if strings.HasSuffix(strings.ToLower(base), "readme") {
			return "(index.html)"
		}
		return "(" + base + ".html)"
	})
}

func ensureStyleSheet(siteDir string) error {
	cssPath := filepath.Join(siteDir, "styles.css")
	_, err := os.Stat(cssPath)
	if err == nil {
		return nil
	}
	if !os.IsNotExist(err) {
		return fmt.Errorf("stat stylesheet: %w", err)
	}

	const css = `:root {
  --bg: #f6f8fb;
  --panel: #ffffff;
  --ink: #0f172a;
  --muted: #475569;
  --line: #dbe3ef;
  --brand: #0b66c3;
  --brand-2: #0a4f96;
}
* { box-sizing: border-box; }
body {
  margin: 0;
  font-family: "Segoe UI", "Helvetica Neue", Arial, sans-serif;
	font-size: 17px;
  background: linear-gradient(180deg, #f7fafc, #f2f6fc 55%, #eef3fb);
  color: var(--ink);
}
header {
  background: linear-gradient(120deg, #0d2748, #0b66c3);
  color: #fff;
  padding: 28px 20px;
}
header h1 {
	margin: 0 0 8px;
	font-size: 36px;
}
.layout {
  display: grid;
	grid-template-columns: 260px minmax(0, 1fr);
	gap: 24px;
	max-width: 1320px;
	margin: 28px auto;
	padding: 0 20px 28px;
}
nav {
  background: var(--panel);
  border: 1px solid var(--line);
  border-radius: 12px;
	padding: 16px;
  height: fit-content;
}
nav a {
  display: block;
  text-decoration: none;
  color: var(--ink);
	padding: 11px 12px;
  border-radius: 8px;
	margin: 6px 0;
	font-weight: 600;
}
nav a:hover, nav a.active {
  background: #eaf3ff;
  color: var(--brand-2);
}
main {
  background: var(--panel);
  border: 1px solid var(--line);
  border-radius: 12px;
	padding: 30px;
	font-size: 1.03rem;
}
h1, h2, h3, h4 {
	line-height: 1.25;
	margin-bottom: 0.6em;
}
h2 { margin-top: 0; font-size: 1.9rem; }
h3 { margin-top: 30px; font-size: 1.5rem; }
p, li { line-height: 1.7; }
main ul, main ol { padding-left: 1.35rem; }
main > *:first-child { margin-top: 0; }
pre {
  background: #0f172a;
  color: #e2e8f0;
  border-radius: 10px;
	padding: 16px;
  overflow-x: auto;
	font-size: 14px;
	line-height: 1.55;
}
.copy-wrap {
	position: relative;
	margin: 14px 0;
}
.copy-wrap pre,
.copy-wrap .code {
	margin: 0;
}
.copy-btn {
	position: absolute;
	top: 10px;
	right: 10px;
	border: 1px solid #3b82f6;
	background: #eff6ff;
	color: #0b4ea2;
	border-radius: 8px;
	font-size: 12px;
	font-weight: 700;
	padding: 6px 10px;
	cursor: pointer;
}
.copy-btn:hover {
	background: #dbeafe;
}
code { font-family: Consolas, "Courier New", monospace; }
table {
  width: 100%;
  border-collapse: collapse;
}
th, td {
  border: 1px solid var(--line);
  padding: 10px;
  text-align: left;
}
@media (max-width: 940px) {
	.layout {
		grid-template-columns: 1fr;
		gap: 14px;
		margin-top: 14px;
	}
	main { padding: 20px; }
}
`
	if err := os.WriteFile(cssPath, []byte(css), 0o644); err != nil {
		return fmt.Errorf("write default stylesheet: %w", err)
	}
	return nil
}

func runServe(port int) error {
	siteDir := filepath.Join("docs", "site")
	if _, err := os.Stat(siteDir); err != nil {
		return fmt.Errorf("site dir missing (%s): run build first: %w", siteDir, err)
	}
	addr := fmt.Sprintf(":%d", port)
	fmt.Printf("serving docs on http://localhost:%d\n", port)
	return http.ListenAndServe(addr, http.FileServer(http.Dir(siteDir)))
}
