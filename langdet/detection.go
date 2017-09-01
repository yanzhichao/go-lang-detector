package langdet

import (
	"encoding/json"
	"github.com/chrisport/go-lang-detector/langdet/internal"
	"github.com/pkg/errors"
	"io"
	"io/ioutil"
	"log"
	"sort"
	"strings"
	"bytes"
)

// the depth of n-gram tokens that are created. if nDepth=1, only 1-letter tokens are created
const nDepth = 4

// DefaultMinimumConfidence is the minimum confidence that a language-match must have to be returned as detected language
var DefaultMinimumConfidence float32 = 0.7

var defaultLanguages = []LanguageComparator{}

var DefaultDetector = Detector{defaultLanguages, DefaultMinimumConfidence}

func init() {
	//TODO remove the file parsing some time in future
	analyzedInput, err := ioutil.ReadFile("default_languages.json")
	if err == nil {
		InitDefaultsFromReader(bytes.NewReader(analyzedInput))
		log.Println("Usage of default json is deprecated, default libraries are provided automatically without json file.\n" +
			"To provide custom defaults, please use InitWithDefault")
		return
	}

	def, err := internal.Asset("default_languages.json")
	if err != nil {
		log.Println("Could not initialize default languages")
	}

	InitDefaultsFromReader(strings.NewReader(string(def)))
}

// InitDefaultsFromReader initializes the default languages with a provided Reader
// containing a Marshaled array of Languages
func InitDefaultsFromReader(reader io.Reader) error {
	lan := []Language{}
	err := json.NewDecoder(reader).Decode(&lan)
	if err != nil {
		return errors.Wrap(err, "Could not process languages from io.Reader.")
	}
	for i := range lan {
		defaultLanguages = append(defaultLanguages, &lan[i])
	}
	return nil
}

// Detector has an array of detectable Languages and methods to determine the closest Language to a text.
type Detector struct {
	Languages         []LanguageComparator
	MinimumConfidence float32
}

// NewDetector returns a new Detector without any language.
// It can be used to add languages selectively.
func NewDetector() Detector {
	return Detector{[]LanguageComparator{}, DefaultMinimumConfidence}
}

// NewDefaultLanguages returns a new Detector with the default languages, if loaded:
// currently: Arabic, English, French, German, Hebrew, Russian, Turkish
func NewDefaultLanguages() Detector {
	return Detector{defaultLanguages, DefaultMinimumConfidence}
}

// Add language analyzes a text and creates a new Language with given name.
// The new language will be detectable afterwards by this Detector instance.
func (d *Detector) AddLanguageFromText(textToAnalyze, languageName string) {
	if len(d.Languages) == 0 {
		d.Languages = make([]LanguageComparator, 0, 0)
	}
	analyzedLanguage := Analyze(textToAnalyze, languageName)
	updatedList := append(d.Languages, &analyzedLanguage)
	d.Languages = updatedList
}

// Add language adds a languageComparator to the list of detectable languages by this Detector instance.
func (d *Detector) AddLanguageComparators(comparators ...LanguageComparator) {
	if d.Languages == nil {
		d.Languages = make([]LanguageComparator, 0, 0)
	}
	for i := range comparators {
		d.Languages = append(d.Languages, comparators[i])
	}
}

// Add language adds a language to the list of detectable languages by this Detector instance.
func (d *Detector) AddLanguage(languages ...Language) {
	if d.Languages == nil {
		d.Languages = make([]LanguageComparator, 0, 0)
	}
	for i := range languages {
		d.Languages = append(d.Languages, &languages[i])
	}
}

// GetClosestLanguage returns the name of the language which is closest to the given text if it is confident enough.
// It returns undefined otherwise. Set detector's MinimumConfidence for customization.
func (d *Detector) GetClosestLanguage(text string) string {
	if d.MinimumConfidence <= 0 || d.MinimumConfidence > 1 {
		d.MinimumConfidence = DefaultMinimumConfidence
	}
	if len(d.Languages) == 0 {
		log.Println("no languages configured for this detector")
		return "undefined"
	}
	lmap := lazyLookupMap(text, nDepth)
	c := d.closestFromTable(lmap, text)

	if len(c) == 0 || c[0].Confidence < asPercent(d.MinimumConfidence) {
		return "undefined"
	}
	return c[0].Name
}

var lazyLookupMap = func(text string, nDepth int) func() map[string]int {
	var rankLookupMap map[string]int
	return func() map[string]int {
		if rankLookupMap == nil {
			occ := CreateOccurenceMap(text, nDepth)
			rankLookupMap = CreateRankLookupMap(occ)
		}
		return rankLookupMap
	}
}

// GetLanguages analyzes a text and returns the DetectionResult of all languages of this detector.
func (d *Detector) GetLanguages(text string) []DetectionResult {
	lazyLookupMap := lazyLookupMap(text, nDepth)
	results := d.closestFromTable(lazyLookupMap, text)
	return results
}

// closestFromTable compares a lookupMap map[token]rank with all languages of this Detector and returns
// an array containing all DetectionResults
func (d *Detector) closestFromTable(lookupMap func() map[string]int, originalInput string) []DetectionResult {
	res := []DetectionResult{}

	for _, language := range d.Languages {
		res = append(res, language.CompareTo(lookupMap, originalInput))
	}

	sort.Sort(ResByConf(res))
	return res
}

// CompareTo calculates the out-of-place distance between two Profiles,
// taking into account only items of mapA, that have a value bigger then 300
func GetDistance(mapA, mapB map[string]int, maxDist int) int {
	var result int
	negMaxDist := (-1) * maxDist
	for key, rankA := range mapA {
		if rankA > 300 {
			continue
		}
		var diff int
		if rankB, ok := mapB[key]; ok {
			diff = rankB - rankA
			if diff > maxDist || diff < negMaxDist {
				diff = maxDist
			} else if diff < 0 {
				diff = diff * (-1)
			}
		} else {
			diff = maxDist
		}
		result += diff
	}
	return result
}

// asPercentage takes a float and returns its value in percent, rounded to 1%
func asPercent(input float32) int {
	return int(input * 100)
}
