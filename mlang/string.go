package mlang

import (
	"fmt"
	"sync"

	"golang.org/x/text/language"
)

// Getter reads a resource in the current language. It is not a widget binding:
// imperative users set text again after a language change; ui.Run rebuilds its
// declarations automatically. A getter can be shared between windows.
type Getter func() string

// Translation is one immutable language/text pair prepared by Text or a language
// helper such as En, ZhCN or Ja.
// Its zero value represents an empty, language-neutral fallback.
type Translation struct {
	tag  language.Tag
	text string
}

// Text defines a translation using a BCP 47 language tag. Invalid tags panic:
// resource definitions are program data. Validate external language preferences
// with SetLanguage, which returns errors instead.
func Text(tag, text string) Translation {
	parsed, err := language.Parse(tag)
	if err != nil {
		panic(fmt.Sprintf("mlang: invalid resource language %q: %v", tag, err))
	}
	return Translation{tag: parsed, text: text}
}

// En defines an English translation.
func En(text string) Translation { return Translation{tag: language.English, text: text} }

// ZhCN defines a Simplified Chinese translation for mainland China.
func ZhCN(text string) Translation { return Text("zh-CN", text) }

// Ja defines a Japanese translation.
func Ja(text string) Translation { return Translation{tag: language.Japanese, text: text} }

// Ko defines a Korean translation.
func Ko(text string) Translation { return Translation{tag: language.Korean, text: text} }

// De defines a German translation.
func De(text string) Translation { return Translation{tag: language.German, text: text} }

// Fr defines a French translation.
func Fr(text string) Translation { return Translation{tag: language.French, text: text} }

// Ru defines a Russian translation.
func Ru(text string) Translation { return Translation{tag: language.Russian, text: text} }

// Ar defines an Arabic translation.
func Ar(text string) Translation { return Translation{tag: language.Arabic, text: text} }

// String defines a resource whose first translation is the fallback. Languages
// are matched using x/text's BCP 47 matcher; unrelated or undetermined languages
// use the fallback. An explicitly empty translation stays empty. Duplicate
// canonical tags panic instead of silently replacing another translation.
//
// Define resources outside repeated UI builds, like parsed SVG sources. No
// global resource registry is maintained: the getter owns its immutable data and
// bounded last-language cache, which are collected when the getter is released.
func String(fallback Translation, translations ...Translation) Getter {
	values := make([]Translation, 1, len(translations)+1)
	values[0] = fallback
	values = append(values, translations...)
	tags := make([]language.Tag, len(values))
	seen := make(map[language.Tag]bool, len(values))
	for i, value := range values {
		if seen[value.tag] {
			panic(fmt.Sprintf("mlang: duplicate resource language %q", value.tag))
		}
		seen[value.tag] = true
		tags[i] = value.tag
	}
	matcher := language.NewMatcher(tags, language.PreferSameScript(true))
	var mu sync.Mutex
	var cachedTag language.Tag
	var cachedText string
	valid := false
	return func() string {
		current.RLock()
		tag := current.tag
		current.RUnlock()
		mu.Lock()
		defer mu.Unlock()
		if !valid || cachedTag != tag {
			_, index, confidence := matcher.Match(tag)
			if tag == language.Und || confidence == language.No {
				index = 0
			}
			cachedTag, cachedText, valid = tag, values[index].text, true
		}
		return cachedText
	}
}
