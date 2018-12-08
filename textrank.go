package main

import (
	"github.com/DavidBelicza/TextRank"
	"github.com/stephane-martin/mailstats/utils"
	"strings"
)

type lang struct {
	stopWords map[string]struct{}
}

func (l *lang) IsStopWord(word string) bool {
	if len(word) <= 3 {
		return true
	}
	norm := utils.Normalize(strings.ToLower(strings.TrimSpace(word)))
	_, ok := l.stopWords[norm]
	return ok
}

func (l *lang) FindRootWord(word string) (bool, string) {
	return false, ""
}

func (l *lang) SetActiveLanguage(code string) {

}

func (l *lang) SetWords(code string, words []string) {

}

var wordSeparators = []rune{
	'?', '!', '.', '“', ' ', ',', '\'', '’', '"', ')', '(', '[', ']', '{', '}', '"', ';', '\n', '>', '<', '%', '@', '&', '=', '#',
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

func TextRank(content, language string) map[string]int {
	var l *lang
	switch language {
	case "english":
		l = &lang{stopWords: stopWordsEnglish}
	case "french":
		l = &lang{stopWords: stopWordsFrench}
	default:
		l = &lang{stopWords: stopWordsEnglish}
	}
	tr := textrank.NewTextRank()
	algo := textrank.NewChainAlgorithm()
	tr.Populate(
		content,
		l,
		rule,
	)
	tr.Ranking(algo)
	words := textrank.FindSingleWords(tr)
	results := make(map[string]int)
	var nbWords int
	for _, word := range words {
		results[word.Word] = word.Qty
		nbWords++
		if nbWords > 9 {
			break
		}
	}
	return results
}