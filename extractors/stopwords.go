package extractors

import (
	"strings"

	set "github.com/deckarep/golang-set"
	"github.com/stephane-martin/mailstats/utils"
)

var StopWordsEnglish = set.NewSet()
var StopWordsFrench = set.NewSet()

func init() {
	enB, err := Asset("data/stopwords-en.txt")
	if err != nil {
		panic(err)
	}
	enF, err := Asset("data/stopwords-fr.txt")
	if err != nil {
		panic(err)
	}
	for _, word := range strings.Split(string(enB), "\n") {
		word = strings.ToLower(strings.TrimSpace(word))
		if len(word) > 0 {
			StopWordsEnglish.Add(utils.Normalize(word))
		}
	}
	for _, word := range strings.Split(string(enF), "\n") {
		word = strings.ToLower(strings.TrimSpace(word))
		if len(word) > 0 {
			StopWordsFrench.Add(utils.Normalize(word))
		}
	}
}
