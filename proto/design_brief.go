package proto

// DesignBrief is what a design is for, settled before it is designed or
// redesigned, the same for a canvas's designs and a project's pages: the
// product, who uses it and for what, its tone as decisions, what the first
// screen is for, products of its kind looked at, what to take away, and
// what is fixed. A canvas keeps its own (Canvas.Brief); a project its, in
// its folder (.enowx/brief.md).
type DesignBrief struct {
	// Product is what it is and its category.
	Product string `json:"product"`
	// Users are who use it and the task they come for.
	Users string `json:"users"`
	// Tone is the look's words turned into decisions: calm or lively,
	// dense or airy, polished or raw; then type, space, and how much of
	// a screen carries color.
	Tone string `json:"tone"`
	// FirstScreen is what the first screen is for (the work itself, or a
	// designed opening), its one focal point and its one main action.
	FirstScreen string `json:"firstScreen"`
	// World is the place or material the product belongs to, which tints
	// its neutrals and gives its accent and pictures.
	World string `json:"world,omitempty"`
	// Signature is the one small detail only this product would have, used
	// the same way wherever it belongs.
	Signature string `json:"signature,omitempty"`
	// Peers are products of its kind looked at: what to keep of them, and
	// what to do differently.
	Peers string `json:"peers,omitempty"`
	// Remove is what to take away: what repeats, explains the obvious or
	// decorates without helping.
	Remove string `json:"remove,omitempty"`
	// Fixed is what stays as it is: brand, devices, assets, the
	// project's design system, technical limits.
	Fixed string `json:"fixed,omitempty"`
}
