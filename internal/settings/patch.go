package settings

import (
	"encoding/json"
	"net"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/yothgewalt/reharvester/internal/harvest"
)

// ValidYear reports whether y is a plausible submission year.
func ValidYear(y int) bool { return y >= 1990 && y <= 2100 }

// ValidMax reports whether n is an acceptable record cap.
func ValidMax(n int) bool { return n >= 1 && n <= 50000 }

// ValidSource reports whether name is a harvest source Open accepts.
func ValidSource(name string) bool {
	return slices.Contains(harvest.Sources, strings.ToLower(strings.TrimSpace(name)))
}

// applyFn validates and, on success, writes one JSON key into s. It returns
// "" on success or a message safe to show a user.
type applyFn func(s *Settings, msg json.RawMessage) string

// webKeys lists every JSON key Apply accepts, one field per web settings key.
var webKeys = map[string]applyFn{
	"addr":               applyAddr,
	"project":            applyProject,
	"source":             applySource,
	"max":                applyMax,
	"delay":              applyDelay,
	"arxivSnapshot":      applySnapshotPath,
	"ollamaUrl":          applyOllamaURL,
	"chatModel":          applyChatModel,
	"embedModel":         applyEmbedModel,
	"snapshotNodes":      applySnapshotNodes,
	"openalexKey":        applyOpenAlexKey,
	"semanticScholarKey": applyS2Key,
	"ollamaKey":          applyOllamaKey,
}

// Apply validates a partial PATCH body against cur and returns the result.
// Any invalid key rejects the whole patch: the returned Settings is then cur,
// unchanged, and errs maps each bad JSON key to a message safe to show a
// user. An unrecognised key is rejected with "not a web setting" rather than
// silently ignored, so a client typo is never mistaken for success.
func Apply(cur Settings, raw map[string]json.RawMessage) (Settings, map[string]string) {
	next := cur
	errs := map[string]string{}
	for key, msg := range raw {
		apply, ok := webKeys[key]
		if !ok {
			errs[key] = "not a web setting"
			continue
		}
		if msg := apply(&next, msg); msg != "" {
			errs[key] = msg
		}
	}
	if len(errs) > 0 {
		return cur, errs
	}
	return next, nil
}

func decodeString(msg json.RawMessage, dst *string) string {
	var v string
	if err := json.Unmarshal(msg, &v); err != nil {
		return "must be a string"
	}
	*dst = v
	return ""
}

func applyAddr(s *Settings, msg json.RawMessage) string {
	var v string
	if errMsg := decodeString(msg, &v); errMsg != "" {
		return errMsg
	}
	v = strings.TrimSpace(v)
	_, port, err := net.SplitHostPort(v)
	if err != nil {
		return "must be a valid host:port"
	}
	if n, err := strconv.Atoi(port); err != nil || n < 1 || n > 65535 {
		return "must be a valid host:port"
	}
	s.Addr = v
	return ""
}

func applyProject(s *Settings, msg json.RawMessage) string {
	var v string
	if errMsg := decodeString(msg, &v); errMsg != "" {
		return errMsg
	}
	v = strings.TrimSpace(v)
	if strings.ContainsAny(v, `/\.`) {
		return `must not contain "/", "\" or "."`
	}
	s.Project = v
	return ""
}

func applySource(s *Settings, msg json.RawMessage) string {
	var v string
	if errMsg := decodeString(msg, &v); errMsg != "" {
		return errMsg
	}
	v = strings.ToLower(strings.TrimSpace(v))
	if !ValidSource(v) {
		return "must be one of " + strings.Join(harvest.Sources, ", ")
	}
	s.Source = v
	return ""
}

func applyMax(s *Settings, msg json.RawMessage) string {
	var v int
	if err := json.Unmarshal(msg, &v); err != nil {
		return "must be a whole number"
	}
	if !ValidMax(v) {
		return "must be between 1 and 50000"
	}
	s.Max = v
	return ""
}

func applyDelay(s *Settings, msg json.RawMessage) string {
	var v string
	if errMsg := decodeString(msg, &v); errMsg != "" {
		return errMsg
	}
	d, err := time.ParseDuration(strings.TrimSpace(v))
	if err != nil || d < 0 || d > time.Minute {
		return "must be a duration between 0 and 1m, like 3s"
	}
	s.Delay = d
	return ""
}

func applySnapshotPath(s *Settings, msg json.RawMessage) string {
	var v string
	if errMsg := decodeString(msg, &v); errMsg != "" {
		return errMsg
	}
	s.SnapshotPath = strings.TrimSpace(v)
	return ""
}

func applyOllamaURL(s *Settings, msg json.RawMessage) string {
	var v string
	if errMsg := decodeString(msg, &v); errMsg != "" {
		return errMsg
	}
	v = strings.TrimSpace(v)
	u, err := url.Parse(v)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return "must be an http or https URL with a host"
	}
	s.OllamaURL = v
	return ""
}

func applyModelName(dst *string, msg json.RawMessage) string {
	var v string
	if errMsg := decodeString(msg, &v); errMsg != "" {
		return errMsg
	}
	v = strings.TrimSpace(v)
	if v == "" {
		return "must not be empty"
	}
	if strings.ContainsFunc(v, unicode.IsSpace) {
		return "must not contain spaces"
	}
	*dst = v
	return ""
}

func applyChatModel(s *Settings, msg json.RawMessage) string {
	return applyModelName(&s.ChatModel, msg)
}

func applyEmbedModel(s *Settings, msg json.RawMessage) string {
	return applyModelName(&s.EmbedModel, msg)
}

func applySnapshotNodes(s *Settings, msg json.RawMessage) string {
	var v int
	if err := json.Unmarshal(msg, &v); err != nil {
		return "must be a whole number"
	}
	if !ValidMax(v) {
		return "must be between 1 and 50000"
	}
	s.Snapshot = v
	return ""
}

// applyKey handles a secret field: a string sets it, null clears it, and ""
// is rejected — a caller that means to clear a key must send null.
func applyKey(dst *string, msg json.RawMessage) string {
	if string(msg) == "null" {
		*dst = ""
		return ""
	}
	var v string
	if errMsg := decodeString(msg, &v); errMsg != "" {
		return errMsg
	}
	if strings.TrimSpace(v) == "" {
		return `must not be "" — send null to clear it`
	}
	*dst = v
	return ""
}

func applyOpenAlexKey(s *Settings, msg json.RawMessage) string { return applyKey(&s.OpenAlexKey, msg) }
func applyS2Key(s *Settings, msg json.RawMessage) string {
	return applyKey(&s.SemanticScholarKey, msg)
}
func applyOllamaKey(s *Settings, msg json.RawMessage) string { return applyKey(&s.OllamaKey, msg) }
