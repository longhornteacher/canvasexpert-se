package workflow

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	stdhtml "html"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"canvasexpertse/internal/outlook"
	"canvasexpertse/internal/workspace"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	goldhtml "github.com/yuin/goldmark/renderer/html"
	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

// FamilyMessageLimit is a conservative local wire-size estimate. The school's
// server or Outlook profile can impose a smaller limit. Base64 expansion of
// every attachment is included, with a small allowance for MIME headers.
const FamilyMessageLimit = 20 * 1024 * 1024

type FamilyRendered struct {
	Subject     string
	HTML        string
	PreviewHTML string
	Digest      string
	Attachments []outlook.Attachment
}

var familyToken = regexp.MustCompile(`\{\{\s*([^{}]+?)\s*\}\}`)
var familyMarkdown = goldmark.New(goldmark.WithExtensions(extension.Table, extension.Strikethrough), goldmark.WithRendererOptions(goldhtml.WithUnsafe()))

// ValidateFamilyContent reports repairable authoring issues without stopping
// preparation. Rendering uses the same neutralized content for preview and mail.
func ValidateFamilyContent(dir string, doc FamilyContact) ([]string, error) {
	var warnings []string
	for _, msg := range doc.Messages {
		_, notes, err := renderFamily(dir, msg, "Student", "Group", "")
		for _, note := range notes {
			warnings = append(warnings, fmt.Sprintf("message %s: %s", msg.Key, note))
		}
		if err != nil {
			return warnings, err
		}
	}
	return warnings, nil
}

func RenderFamilyMessage(dir string, msg FamilyMessage, studentName, groupName, signOff string) (FamilyRendered, error) {
	rendered, _, err := renderFamily(dir, msg, studentName, groupName, signOff)
	return rendered, err
}

type familyAsset struct {
	attachment outlook.Attachment
	data       []byte
}

func renderFamily(dir string, msg FamilyMessage, studentName, groupName, signOff string) (FamilyRendered, []string, error) {
	var out FamilyRendered
	var notes []string
	dir, err := filepath.Abs(dir)
	if err != nil {
		return out, notes, errors.New("family run folder is unavailable")
	}
	for _, match := range familyToken.FindAllStringSubmatch(msg.Subject+"\n"+msg.Body, -1) {
		if match[1] != "student_name" && match[1] != "group_name" {
			notes = append(notes, "unknown placeholder {{"+match[1]+"}} left unchanged")
		}
	}
	substitute := func(s string) string {
		return familyToken.ReplaceAllStringFunc(s, func(token string) string {
			switch strings.TrimSpace(familyToken.FindStringSubmatch(token)[1]) {
			case "student_name":
				return studentName
			case "group_name":
				return groupName
			}
			return token
		})
	}
	out.Subject = substitute(msg.Subject)
	// Substitute on parsed text nodes, after Markdown conversion, so names cannot
	// introduce Markdown formatting, HTML, or a new image/link destination.
	source := msg.Body
	var body string
	switch strings.ToLower(strings.TrimSpace(msg.Format)) {
	case "", "markdown":
		var buf bytes.Buffer
		if err := familyMarkdown.Convert([]byte(source), &buf); err != nil {
			return out, notes, errors.New("family message could not be rendered")
		}
		body = buf.String()
	case "html":
		body = source
	default:
		return out, notes, errors.New("family message format must be markdown or html")
	}
	if signOff = strings.TrimSpace(signOff); signOff != "" && !familyHasSignOff(source, signOff) {
		body += "<p>" + strings.ReplaceAll(stdhtml.EscapeString(signOff), "\n", "<br>") + "</p>"
	}
	nodes, err := html.ParseFragment(strings.NewReader(body), &html.Node{Type: html.ElementNode, Data: "div", DataAtom: atom.Div})
	if err != nil {
		return out, notes, errors.New("family message HTML could not be read")
	}
	root := &html.Node{Type: html.ElementNode, Data: "div", DataAtom: atom.Div}
	for _, n := range nodes {
		root.AppendChild(n)
	}
	assets := map[string]*familyAsset{}
	var ordered []*familyAsset
	var attachmentBytes int64
	load := func(ref string, inline bool) *familyAsset {
		path, err := familyAssetPath(dir, ref)
		if err != nil {
			notes = append(notes, "missing or unsafe asset omitted; use a local file under assets/")
			return nil
		}
		if a := assets[path]; a != nil {
			if inline && !familyImageType(a.attachment.ContentType) {
				notes = append(notes, "unsupported inline image omitted; use PNG, JPEG, GIF, BMP or WebP")
				return nil
			}
			if inline && a.attachment.ContentID == "" {
				a.attachment.ContentID = familyCID(path)
			}
			return a
		}
		info, err := os.Stat(path)
		if err != nil || !info.Mode().IsRegular() {
			notes = append(notes, "missing asset omitted; save the file under assets/")
			return nil
		}
		// Check before reading to avoid allocating oversized local files.
		if info.Size() > FamilyMessageLimit || attachmentBytes+info.Size() > FamilyMessageLimit {
			attachmentBytes = FamilyMessageLimit + 1
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			notes = append(notes, "unreadable asset omitted")
			return nil
		}
		contentType := http.DetectContentType(data)
		if inline && !familyImageType(contentType) {
			notes = append(notes, "unsupported inline image omitted; use PNG, JPEG, GIF, BMP or WebP")
			return nil
		}
		a := &familyAsset{attachment: outlook.Attachment{Path: path, Name: filepath.Base(path), ContentType: contentType}, data: data}
		if inline {
			a.attachment.ContentID = familyCID(path)
		}
		assets[path] = a
		ordered = append(ordered, a)
		attachmentBytes += int64(len(data))
		return a
	}
	var clean func(*html.Node)
	clean = func(n *html.Node) {
		for child := n.FirstChild; child != nil; {
			next := child.NextSibling
			if child.Type == html.TextNode {
				child.Data = substitute(child.Data)
			} else if child.Type != html.ElementNode {
				n.RemoveChild(child)
			} else if !familyAllowedTag(child.Data) {
				// Drop executable/remote-loading subtrees outright. For unfamiliar
				// ordinary containers preserve their readable text/content.
				if familyDropTag(child.Data) {
					n.RemoveChild(child)
				} else {
					clean(child)
					for child.FirstChild != nil {
						c := child.FirstChild
						child.RemoveChild(c)
						n.InsertBefore(c, child)
					}
					n.RemoveChild(child)
				}
			} else {
				var attrs []html.Attribute
				for _, attr := range child.Attr {
					if attr.Namespace != "" {
						continue
					}
					switch attr.Key {
					case "alt", "title":
						attr.Val = substitute(attr.Val)
						attrs = append(attrs, attr)
					case "width", "height", "colspan", "rowspan":
						if familyDimension.MatchString(attr.Val) {
							attrs = append(attrs, attr)
						}
					case "style":
						if value := familyStyle(attr.Val); value != "" {
							attr.Val = value
							attrs = append(attrs, attr)
						}
					case "href":
						if child.Data == "a" && familySafeLink(attr.Val) {
							attrs = append(attrs, attr)
						} else if child.Data == "a" && strings.HasPrefix(attr.Val, "assets/") {
							load(attr.Val, false)
						}
					}
				}
				if child.Data == "img" {
					ref, alt := "", ""
					for _, attr := range child.Attr {
						if attr.Key == "src" {
							ref = attr.Val
						}
						if attr.Key == "alt" {
							alt = substitute(attr.Val)
						}
					}
					var asset *familyAsset
					if strings.HasPrefix(strings.ToLower(strings.TrimSpace(ref)), "http:") || strings.HasPrefix(strings.ToLower(strings.TrimSpace(ref)), "https:") || strings.HasPrefix(ref, "//") {
						notes = append(notes, "remote image omitted; save it under assets/")
					} else {
						asset = load(ref, true)
					}
					if asset == nil {
						n.InsertBefore(&html.Node{Type: html.TextNode, Data: alt}, child)
						n.RemoveChild(child)
						child = next
						continue
					}
					attrs = append(attrs, html.Attribute{Key: "src", Val: "cid:" + asset.attachment.ContentID})
				}
				child.Attr = attrs
				clean(child)
			}
			child = next
		}
	}
	clean(root)
	for _, ref := range msg.Attachments {
		load(ref, false)
	}
	var buf bytes.Buffer
	for n := root.FirstChild; n != nil; n = n.NextSibling {
		if err := html.Render(&buf, n); err != nil {
			return out, notes, errors.New("family message could not be rendered")
		}
	}
	out.HTML = buf.String()
	encodedBytes := ((attachmentBytes + 2) / 3) * 4
	// MIME base64 lines wrap at 76 characters with CRLF. Include that wire
	// overhead as well as per-part and ordinary message headers.
	wireBytes := int64(len(out.HTML)) + encodedBytes + ((encodedBytes+75)/76)*2 + int64(len(ordered))*2048 + 4096
	if wireBytes > FamilyMessageLimit {
		return out, notes, errors.New("family message exceeds the 20 MiB delivery estimate; reduce the body or assets")
	}
	out.PreviewHTML = out.HTML
	digest := sha256.New()
	for _, value := range []string{out.Subject, out.HTML} {
		fmt.Fprintf(digest, "%d:", len(value))
		digest.Write([]byte(value))
	}
	for _, a := range ordered {
		out.Attachments = append(out.Attachments, a.attachment)
		for _, value := range []string{a.attachment.Name, a.attachment.ContentID, a.attachment.ContentType} {
			fmt.Fprintf(digest, "%d:", len(value))
			digest.Write([]byte(value))
		}
		fmt.Fprintf(digest, "%d:", len(a.data))
		digest.Write(a.data)
		if a.attachment.ContentID != "" {
			out.PreviewHTML = strings.ReplaceAll(out.PreviewHTML, `src="cid:`+a.attachment.ContentID+`"`, `src="data:`+a.attachment.ContentType+`;base64,`+base64.StdEncoding.EncodeToString(a.data)+`"`)
		}
	}
	out.Digest = hex.EncodeToString(digest.Sum(nil))
	return out, notes, nil
}

func familyCID(path string) string {
	digest := sha256.Sum256([]byte(path))
	return fmt.Sprintf("cse-%x@family", digest[:12])
}
func familyImageType(t string) bool {
	return t == "image/png" || t == "image/jpeg" || t == "image/gif" || t == "image/bmp" || t == "image/webp"
}
func familyAssetPath(dir, ref string) (string, error) {
	if strings.ContainsAny(ref, "\\\x00") {
		return "", errors.New("unsafe asset")
	}
	u, err := url.Parse(ref)
	if err != nil || u.IsAbs() || u.Host != "" || u.RawQuery != "" || u.Fragment != "" {
		return "", errors.New("unsafe asset")
	}
	if !strings.HasPrefix(u.Path, "assets/") {
		return "", errors.New("unsafe asset")
	}
	// Constrain both the assets directory and its contents: this also resolves
	// Windows directory junctions, which EvalSymlinks alone does not follow.
	root, err := workspace.ConstrainPath(dir, "assets")
	if err != nil {
		return "", err
	}
	if _, err := workspace.ConstrainPath(dir, u.Path); err != nil {
		return "", err
	}
	return workspace.ConstrainPath(root, strings.TrimPrefix(u.Path, "assets/"))
}

func familyAllowedTag(tag string) bool {
	return strings.Contains(" p div span br hr h1 h2 h3 h4 h5 h6 ul ol li blockquote pre code strong b em i u s del table thead tbody tfoot tr th td a img sup sub ", " "+tag+" ")
}
func familyDropTag(tag string) bool {
	return strings.Contains(" script style iframe frame frameset object embed link meta base svg math form input button textarea select option video audio source track canvas template ", " "+tag+" ")
}

var familyDimension = regexp.MustCompile(`^[0-9]{1,4}%?$`)
var familyStyleValue = regexp.MustCompile(`^[a-zA-Z0-9#.,% ()+-]+$`)

func familyStyle(style string) string {
	var safe []string
	for _, declaration := range strings.Split(style, ";") {
		pair := strings.SplitN(declaration, ":", 2)
		if len(pair) != 2 {
			continue
		}
		key, value := strings.ToLower(strings.TrimSpace(pair[0])), strings.TrimSpace(pair[1])
		if !strings.Contains(" color background-color font-size font-family font-weight font-style text-decoration text-align line-height margin margin-top margin-bottom padding border border-collapse width max-width height ", " "+key+" ") || !familyStyleValue.MatchString(value) {
			continue
		}
		lower := strings.ToLower(value)
		if strings.Contains(lower, "url") || strings.Contains(lower, "expression") || strings.Contains(lower, "var(") {
			continue
		}
		safe = append(safe, key+":"+value)
	}
	return strings.Join(safe, ";")
}
func familySafeLink(value string) bool {
	u, err := url.Parse(strings.TrimSpace(value))
	if err != nil {
		return false
	}
	return strings.EqualFold(u.Scheme, "http") || strings.EqualFold(u.Scheme, "https") || strings.EqualFold(u.Scheme, "mailto") || (u.Scheme == "" && strings.HasPrefix(value, "#"))
}
func familyHasSignOff(body, signOff string) bool {
	plain := regexp.MustCompile(`<[^>]*>`).ReplaceAllString(body, " ")
	plain = strings.NewReplacer("*", "", "_", "", "\r", " ").Replace(plain)
	normalize := func(s string) string { return strings.Join(strings.Fields(strings.ToLower(s)), " ") }
	return strings.HasSuffix(normalize(plain), normalize(signOff))
}
