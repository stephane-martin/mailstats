package extractors

import (
	"fmt"
	"github.com/DavidBelicza/TextRank"
	"github.com/DavidBelicza/TextRank/rank"
	"github.com/stephane-martin/mailstats/utils"
	"strings"
	"unicode/utf8"
)

type lang struct {
	stopWords func(string) bool
	stems map[string]string
}

func (l *lang) IsStopWord(word string) bool {
	norm := utils.Normalize(strings.ToLower(strings.TrimSpace(word)))
	if utf8.RuneCountInString(norm) <= 3 {
		return true
	}
	return l.stopWords(norm)
}

func (l *lang) FindRootWord(word string) (bool, string) {
	norm := utils.Normalize(strings.ToLower(strings.TrimSpace(word)))
	if w, ok := l.stems[norm]; ok {
		return true, w
	}
	return false, ""
}

func (l *lang) SetActiveLanguage(code string) {

}

func (l *lang) SetWords(code string, words []string) {

}

var wordSeparators = []rune{
	'«', '»', '+', '/', ':', '-', '_', '?', '!', '.', '“', ' ', ',', '\'', '’', '"', ')', '(', '[', ']', '{', '}', '"', ';', '\n', '>', '<', '%', '@', '&', '=', '#',
}
var wordsSeparatorsMap map[rune]struct{}

var sentencesSeparatorsMap = map[rune]struct{} {
	'.': {},
	'!': {},
	'?': {},
}

func init() {
	wordsSeparatorsMap = make(map[rune]struct{})
	for _, sep := range wordSeparators {
		wordsSeparatorsMap[sep] = struct{}{}
	}
}

type rulet struct {}

func (r rulet) IsWordSeparator(rune rune) bool {
	_, ok := wordsSeparatorsMap[rune]
	return ok
}

func (r rulet) IsSentenceSeparator(rune rune) bool {
	_, ok := sentencesSeparatorsMap[rune]
	return ok
}

var rule rulet

func TextRank(content string, stems map[string]string, language string) ([]rank.SingleWord, []rank.Phrase) {
	content = strings.TrimSpace(content)
	if content == "" {
		return nil, nil
	}
	if len(stems) == 0 {
		stems = Stems(BagOfWords(content, language), language)
	}
	var stopWords map[string]struct{}
	if language == "french" {
		stopWords = StopWordsFrench
	} else {
		stopWords = StopWordsEnglish
	}

	l := &lang{
		stopWords: func(w string) bool {
			_, ok := stopWords[w]
			return ok
		},
		stems: stems,
	}
	tr := textrank.NewTextRank()
	algo := textrank.NewChainAlgorithm()
	tr.Populate(
		content,
		l,
		rule,
	)
	tr.Ranking(algo)
	return textrank.FindSingleWords(tr), textrank.FindPhrases(tr)
}

func Keywords(content string, stems map[string]string, language string) ([]string, []string) {
	if language == "" {
		language = Language(content)
	}
	trKW, trPh := TextRank(content, stems, language)
	keywords := make([]string, 0, 10)
	phrases := make([]string, 0, 10)
	var nbWords int
	for _, w := range trKW {
		keywords = append(keywords, w.Word)
		nbWords++
		if nbWords == 10 {
			break
		}
	}
	nbWords = 0
	for _, ph := range trPh {
		phrases = append(phrases, fmt.Sprintf("%s/%s", ph.Left, ph.Right))
		nbWords++
		if nbWords == 10 {
			break
		}
	}
	return keywords, phrases
}