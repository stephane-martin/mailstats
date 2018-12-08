package extractors

import (
	"github.com/stephane-martin/mailstats/utils"
	"strings"
)

var StopWordsEnglish map[string]struct{}
var StopWordsFrench map[string]struct{}

func init() {
	enB, err := Asset("data/stopwords-en.txt")
	if err != nil {
		panic(err)
	}
	enF, err := Asset("data/stopwords-fr.txt")
	if err != nil {
		panic(err)
	}
	StopWordsEnglish = make(map[string]struct{})
	StopWordsFrench = make(map[string]struct{})
	for _, word := range strings.Split(string(enB), "\n") {
		word = strings.ToLower(strings.TrimSpace(word))
		if len(word) > 0 {
			StopWordsEnglish[utils.Normalize(word)] = struct{}{}
		}
	}
	for _, word := range strings.Split(string(enF), "\n") {
		word = strings.ToLower(strings.TrimSpace(word))
		if len(word) > 0 {
			StopWordsFrench[utils.Normalize(word)] = struct{}{}
		}
	}
}
