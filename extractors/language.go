package extractors

import (
	"github.com/abadojack/whatlanggo"
	"strings"
)

func Language(content string) string {
	langInfo := whatlanggo.Detect(content)
	if langInfo.Script != nil && langInfo.Lang != -1 {
		return strings.ToLower(whatlanggo.Langs[langInfo.Lang])
	}
	return ""
}