package designcompare

import (
	"errors"
	"regexp"
	"strings"

	"github.com/enowx/enowx/proto"
)

// A direction's card as text, a "Field: value" line each, so agents write
// it the way people read it.

type cardField struct {
	name     string
	required bool
	of       func(*proto.DesignDirection) *string
}

var cardFields = []cardField{
	{"North star", true, func(d *proto.DesignDirection) *string { return &d.NorthStar }},
	{"Source", true, func(d *proto.DesignDirection) *string { return &d.Source }},
	{"Skeleton", true, func(d *proto.DesignDirection) *string { return &d.Skeleton }},
	{"Type", true, func(d *proto.DesignDirection) *string { return &d.Type }},
	{"Interaction", true, func(d *proto.DesignDirection) *string { return &d.Interaction }},
	{"Hero", false, func(d *proto.DesignDirection) *string { return &d.Hero }},
	{"Icons", false, func(d *proto.DesignDirection) *string { return &d.Icons }},
	{"Controls", false, func(d *proto.DesignDirection) *string { return &d.Controls }},
	{"Moment", false, func(d *proto.DesignDirection) *string { return &d.Moment }},
	{"Break", false, func(d *proto.DesignDirection) *string { return &d.Break }},
	{"Gives up", false, func(d *proto.DesignDirection) *string { return &d.GivesUp }},
}

// CardFormat is how a card is written.
const CardFormat = `North star: the direction in a sentence
Source: the concrete thing it grows from (a material and where it's used, a scene with its time and light, another field's visual language, a convention broken on purpose), never an adjective
Skeleton: the first screen's layout, its focal point and main action, where reading starts
Type: families, weights, scale (display, body, numbers)
Interaction: the main action: from where, what changes, what shows, the state after
Palette: hex colors, the background first, then the accent, then selected, border and disabled (three at the least), and where in the product they come from (not the source's material)
Hero: the main picture and where it comes from (reused, made, or the type itself)
Icons: the set and stroke, or none
Controls: radius, stroke and press, in a sentence
Moment: the one memorable moment (a reaction to an action, a success, work under way, an empty or error state)
Break: the one place it departs from the category's habit, in the structure (not a decoration)
Gives up: what it leaves out on purpose`

var hexRe = regexp.MustCompile(`#[0-9a-fA-F]{6}\b|#[0-9a-fA-F]{3}\b`)

// ParseCard reads a card written in CardFormat.
func ParseCard(text string) (proto.DesignDirection, error) {
	var d proto.DesignDirection
	for line := range strings.SplitSeq(text, "\n") {
		key, v, ok := strings.Cut(strings.TrimLeft(strings.TrimSpace(line), "-*• "), ":")
		if !ok {
			continue
		}
		key, v = strings.ToLower(strings.TrimSpace(key)), strings.TrimSpace(v)
		if key == "palette" {
			d.Palette = hexRe.FindAllString(v, -1)
			continue
		}
		for _, f := range cardFields {
			if strings.ToLower(f.name) == key {
				*f.of(&d) = v
			}
		}
	}
	var missing []string
	for _, f := range cardFields {
		if f.required && *f.of(&d) == "" {
			missing = append(missing, f.name)
		}
	}
	if len(d.Palette) < 3 {
		missing = append(missing, "Palette (three hex colors at the least)")
	}
	if len(missing) > 0 {
		return d, errors.New("the card has no " + strings.Join(missing, ", "))
	}
	return d, nil
}

// CardText is card d in CardFormat.
func CardText(d proto.DesignDirection) string {
	var b strings.Builder
	for i, f := range cardFields {
		if i == 5 && len(d.Palette) > 0 {
			b.WriteString("Palette: " + strings.Join(d.Palette, ", ") + "\n")
		}
		if v := *f.of(&d); v != "" {
			b.WriteString(f.name + ": " + v + "\n")
		}
	}
	return b.String()
}
