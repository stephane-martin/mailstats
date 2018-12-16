package extractors

import (
	"github.com/abadojack/whatlanggo"
	"github.com/chrisport/go-lang-detector/langdet"
	"github.com/chrisport/go-lang-detector/langdet/langdetdef"
	"strings"
)

var whitelist = whatlanggo.Options{
	Whitelist: map[whatlanggo.Lang]bool{
		whatlanggo.Fra: true,
		whatlanggo.Eng: true,
		whatlanggo.Deu: true,
		whatlanggo.Rus: true,
	},
}

func Language(content string) string {
	lang := whatlanggo.DetectLangWithOptions(content, whitelist)
	if lang != -1 {
		return strings.ToLower(whatlanggo.Langs[lang])
	}
	return ""
}

func Language2(content string) string {
	detector := langdet.Detector{
		Languages: []langdet.LanguageComparator{
			langdetdef.ENGLISH,
			langdetdef.RUSSIAN,
			langdetdef.GERMAN,
			langdetdef.FRENCH,
		},
		MinimumConfidence: langdet.DefaultMinimumConfidence,
		NDepth:            langdet.DEFAULT_NDEPTH,
	}
	return strings.ToLower(detector.GetClosestLanguage(content))
}
