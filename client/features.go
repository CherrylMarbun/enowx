package client

import "github.com/enowx/enowx/proto"

// Features lists the features and whether each is on for p.
func (c *Client) Features(p proto.FeaturesParams) (proto.FeaturesState, error) {
	var st proto.FeaturesState
	err := c.Call(proto.MethodFeaturesList, p, &st)
	return st, err
}

// SetFeature turns a feature on or off in a scope.
func (c *Client) SetFeature(p proto.FeatureSetParams) (proto.FeaturesState, error) {
	var st proto.FeaturesState
	err := c.Call(proto.MethodFeaturesSet, p, &st)
	return st, err
}

// SetFeaturesLazy turns lazy guides on or off.
func (c *Client) SetFeaturesLazy(lazy bool) (proto.FeaturesState, error) {
	var st proto.FeaturesState
	err := c.Call(proto.MethodFeaturesLazy, proto.FeaturesLazyParams{Lazy: lazy}, &st)
	return st, err
}

// FeaturesPreset sets a scope's switches to a preset's.
func (c *Client) FeaturesPreset(p proto.FeaturesPresetParams) (proto.FeaturesState, error) {
	var st proto.FeaturesState
	err := c.Call(proto.MethodFeaturesPreset, p, &st)
	return st, err
}

// FeaturesGuide is the guide a session (or a project's) gets.
func (c *Client) FeaturesGuide(p proto.FeaturesParams) (proto.FeaturesGuide, error) {
	var g proto.FeaturesGuide
	err := c.Call(proto.MethodFeaturesGuide, p, &g)
	return g, err
}

// RequestFeature asks the user to turn a feature on, and waits for them.
func (c *Client) RequestFeature(r proto.FeatureRequest) (proto.FeatureAnswer, error) {
	var a proto.FeatureAnswer
	err := c.Call(proto.MethodFeaturesRequest, r, &a)
	return a, err
}

// AnswerFeature answers an agent's request with the user's answer.
func (c *Client) AnswerFeature(a proto.FeatureAnswer) error {
	return c.Call(proto.MethodFeaturesAnswer, a, nil)
}
