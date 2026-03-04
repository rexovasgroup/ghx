package api

import (
	"bytes"
	"encoding/json"
)

// ReactionGroups is a slice of ReactionGroup entries for an issue or comment.
type ReactionGroups []ReactionGroup

// MarshalJSON serializes ReactionGroups to JSON, omitting groups with zero reactions.
func (rg ReactionGroups) MarshalJSON() ([]byte, error) {
	buf := bytes.Buffer{}
	buf.WriteRune('[')
	encoder := json.NewEncoder(&buf)
	encoder.SetEscapeHTML(false)

	hasPrev := false
	for _, g := range rg {
		if g.Users.TotalCount == 0 {
			continue
		}
		if hasPrev {
			buf.WriteRune(',')
		}
		if err := encoder.Encode(&g); err != nil {
			return nil, err
		}
		hasPrev = true
	}
	buf.WriteRune(']')
	return buf.Bytes(), nil
}

// ReactionGroup represents a single emoji reaction type and its associated users.
type ReactionGroup struct {
	Content string             `json:"content"`
	Users   ReactionGroupUsers `json:"users"`
}

// ReactionGroupUsers holds the total count of users who reacted with a particular emoji.
type ReactionGroupUsers struct {
	TotalCount int `json:"totalCount"`
}

// Count returns the total number of users who reacted with this emoji.
func (rg ReactionGroup) Count() int {
	return rg.Users.TotalCount
}

// Emoji returns the Unicode emoji character for this reaction group's content type.
func (rg ReactionGroup) Emoji() string {
	return reactionEmoji[rg.Content]
}

var reactionEmoji = map[string]string{
	"THUMBS_UP":   "\U0001f44d",
	"THUMBS_DOWN": "\U0001f44e",
	"LAUGH":       "\U0001f604",
	"HOORAY":      "\U0001f389",
	"CONFUSED":    "\U0001f615",
	"HEART":       "\u2764\ufe0f",
	"ROCKET":      "\U0001f680",
	"EYES":        "\U0001f440",
}
