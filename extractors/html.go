package extractors

import (
	"github.com/jaytaylor/html2text"
	"golang.org/x/net/html"
	"net/url"
	"strings"
)

func HTML2Text(h string) (text string, links []string, images []string) {
	h = strings.TrimSpace(h)
	if len(h) == 0 {
		return "", nil, nil
	}
	z := html.NewTokenizer(strings.NewReader(h))
	for {
		t := z.Next()
		if t == html.ErrorToken {
			break
		}
		if t == html.StartTagToken {
			tok := z.Token()
			tagName := strings.ToLower(tok.Data)
			if tagName == "a" {
				for _, attr := range tok.Attr {
					if strings.ToLower(attr.Key) == "href" {
						v, err := url.PathUnescape(attr.Val)
						if err == nil {
							links = append(links, v)
						}
						break
					}
				}
			}
			if tagName == "img" {
				for _, attr := range tok.Attr {
					if strings.ToLower(attr.Key) == "src" {
						v, err := url.PathUnescape(attr.Val)
						if err == nil {
							images = append(images, v)
						}
						break
					}
				}
			}
		}
	}
	t, err := html2text.FromString(h)
	if err != nil {
		t = ""
	}
	return strings.TrimSpace(t), links, images
}
