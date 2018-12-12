package extractors

import (
	"github.com/jdkato/prose"
	"github.com/kljensen/snowball"
	"github.com/stephane-martin/mailstats/utils"
	"strings"
	"unicode/utf8"
)

func ExtractWords(text string) (words []string) {
	text = strings.TrimSpace(text)
	if len(text) == 0 {
		return nil
	}
	doc, err := prose.NewDocument(text, prose.WithExtraction(false), prose.WithSegmentation(false), prose.WithTagging(false))
	if err != nil {
		return nil
	}
	for _, tok := range doc.Tokens() {
		word := filterWord(tok.Text)
		if len(word) > 0 {
			words = append(words, word)
		}
	}
	return words
}

var abbrs = []string{"t'", "s'", "d'", "j'", "l'", "m'", "c'", "n'", "qu'", "«"}

func filterWord(w string) string {
	w = strings.ToLower(utils.Normalize(w))
	for _, abbr := range abbrs {
		for {
			w2 := strings.TrimPrefix(w, abbr)
			if w2 == w {
				break
			}
			w = w2
		}
	}
	w = strings.TrimSuffix(w, "»")
	if utf8.RuneCountInString(w) <= 3 {
		return ""
	}
	if strings.ContainsAny(w, "«<>0123456789_-~#{}[]()|`^=+°&$£µ%/:;,?§!.@") {
		return ""
	}
	return w
}

func BagOfWords(text string, language string) map[string]int {
	bag := make(map[string]int)
	words := ExtractWords(text)
	if len(words) == 0 {
		return bag
	}
	for _, word := range words {
		bag[word] = bag[word] + 1
	}
	switch language {
	case "english":
		for word := range bag {
			if _, ok := StopWordsEnglish[word]; ok {
				delete(bag, word)
			}
		}
	case "french":
		for word := range bag {
			if _, ok := StopWordsFrench[word]; ok {
				delete(bag, word)
			}
		}
	default:
	}
	return bag
}

func Stems(bag map[string]int, language string) map[string]string {
	s := make(map[string][]string)
	for word := range bag {
		stem, err := snowball.Stem(word, language, false)
		if err == nil {
			s[stem] = append(s[stem], word)
		} else {
			s[word] = append(s[word], word)
		}
	}
	stems := map[string]string{}
	for _, words := range s {
		shortest := shortestWord(words)
		for _, word := range words {
			stems[word] = shortest
		}
	}
	return stems
}

func shortestWord(words []string) string {
	if len(words) == 0 {
		return ""
	}
	shortest := words[0]
	for _, word := range words {
		if utf8.RuneCountInString(word) < utf8.RuneCountInString(shortest) {
			shortest = word
			continue
		}
		if utf8.RuneCountInString(word) == utf8.RuneCountInString(shortest) && word < shortest {
			shortest = word
		}
	}
	return shortest
}
