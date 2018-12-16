package extractors

import (
	"golang.org/x/net/html"
	"net/url"
	"strings"
)

func HTML2Text(h string) (text string, links []string, images []string) {
	h = strings.TrimSpace(h)
	if len(h) == 0 {
		return "", nil, nil
	}
	texts := make([]string, 0)
	z := html.NewTokenizer(strings.NewReader(h))
	ignoreScript := false
	ignoreNoScript := false
	ignoreStyle := false
	for {
		t := z.Next()
		if t == html.ErrorToken {
			break
		} else if t == html.StartTagToken {
			tok := z.Token()
			tagName := strings.ToLower(tok.Data)
			if tagName == "script" {
				ignoreScript = true
			} else if tagName == "noscript" {
				ignoreNoScript = true
			} else if tagName == "style" {
				ignoreStyle = true
			} else if tagName == "a" {
				for _, attr := range tok.Attr {
					if strings.ToLower(attr.Key) == "href" {
						v, err := url.PathUnescape(attr.Val)
						if err == nil {
							links = append(links, v)
						}
						break
					}
				}
			} else if tagName == "img" {
				for _, attr := range tok.Attr {
					if strings.ToLower(attr.Key) == "src" {
						v, err := url.PathUnescape(attr.Val)
						if err == nil {
							images = append(images, v)
						}
						break
					}
				}
			} else {
			}
		} else if t == html.EndTagToken {
			tok := z.Token()
			tagName := strings.ToLower(tok.Data)
			if tagName == "script" {
				ignoreScript = false
			} else if tagName == "noscript" {
				ignoreNoScript = false
			} else if tagName == "style" {
				ignoreStyle = false
			} else if tagName == "title" || tagName == "h1" || tagName == "h2" || tagName == "h3" || tagName == "h4" || tagName == "h5" || tagName == "h6" {
				texts = append(texts, ".")
			}
		} else if t == html.TextToken && !ignoreScript && !ignoreStyle && !ignoreNoScript {
			tok := z.Token()
			textData := strings.TrimSpace(tok.Data)
			if len(textData) > 0 {
				texts = append(texts, textData)
			}
		}
	}
	return strings.TrimSpace(strings.Join(texts, " ")), links, images
}
