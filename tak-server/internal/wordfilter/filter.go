// Package wordfilter detects and masks profanity in usernames and chat.
//
// The word list is a CSV with one word per line and an optional second column
// of underscore-separated exceptions: a word is not flagged when the text also
// contains one of its exceptions (e.g. "ass" is fine inside "bass").
package wordfilter

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
)

// Filter is safe for concurrent use. The zero value flags nothing.
type Filter struct {
	mu    sync.RWMutex
	words map[string][]string // word -> exceptions
}

// Load replaces the word list with the CSV at url and returns the number of
// words loaded.
func (f *Filter) Load(ctx context.Context, url string) (int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return 0, fmt.Errorf("load word list: %w", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0, fmt.Errorf("load word list: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("load word list: %s", resp.Status)
	}
	return f.Read(resp.Body)
}

// Read replaces the word list with CSV read from r and returns the number of
// words loaded.
func (f *Filter) Read(r io.Reader) (int, error) {
	words := make(map[string][]string)
	sc := bufio.NewScanner(r)
	for sc.Scan() {
		word, exceptions, _ := strings.Cut(sc.Text(), ",")
		word = strings.ToLower(strings.ReplaceAll(word, " ", ""))
		if word == "" {
			continue
		}
		exceptions, _, _ = strings.Cut(exceptions, ",")
		var list []string
		if exceptions != "" {
			list = strings.Split(strings.ToLower(exceptions), "_")
		}
		words[word] = list
	}
	if err := sc.Err(); err != nil {
		return 0, fmt.Errorf("read word list: %w", err)
	}
	f.mu.Lock()
	f.words = words
	f.mu.Unlock()
	return len(words), nil
}

// ContainsBadWord checks a username, ignoring spaces and underscores so that
// words can't be hidden by splitting them up.
func (f *Filter) ContainsBadWord(s string) bool {
	return f.found(strings.NewReplacer(" ", "", "_", "").Replace(s))
}

// Mask replaces each space-separated word of s that contains profanity with
// "****".
func (f *Filter) Mask(s string) string {
	parts := strings.Split(s, " ")
	for i, p := range parts {
		if f.found(p) {
			parts[i] = "****"
		}
	}
	return strings.Join(parts, " ")
}

// leetspeak undoes common character substitutions before matching.
var leetspeak = strings.NewReplacer("1", "i", "!", "i", "3", "e", "4", "a", "@", "a", "5", "s", "7", "t", "0", "o", "9", "g", "$", "s")

func (f *Filter) found(s string) bool {
	s = leetspeak.Replace(strings.ToLower(s))
	f.mu.RLock()
	defer f.mu.RUnlock()
	for word, exceptions := range f.words {
		if strings.Contains(s, word) && !containsAny(s, exceptions) {
			return true
		}
	}
	return false
}

func containsAny(s string, subs []string) bool {
	for _, sub := range subs {
		if sub != "" && strings.Contains(s, sub) {
			return true
		}
	}
	return false
}
