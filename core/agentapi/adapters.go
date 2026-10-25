package agentapi

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// Each agent's way of taking an API, from its documentation (docs/agent-api.md
// says where). What enowX adds it names "enowx", so it finds it again and
// leaves the user's own providers alone.

const ourName = "enowx"

func init() {
	adapters["claude"] = claudeCode
	adapters["codex"] = codex
	adapters["opencode"] = openCodeLike("opencode", ".config/opencode/opencode.json")
	adapters["kilo"] = openCodeLike("kilo", ".config/kilo/kilo.json")
	adapters["gemini"] = geminiCLI
	adapters["antigravity"] = noModel(envOnly("antigravity", []string{Gemini}, func(api API) [][2]string {
		return [][2]string{{"GEMINI_API_KEY", api.APIKey}, {"GOOGLE_GEMINI_BASE_URL", api.BaseURL}}
	}, "Antigravity speaks Gemini's API only, with its own models; the sessions enowX starts of it get it."))
	adapters["qwen-code"] = qwenCode
	adapters["droid"] = droid
	adapters["goose"] = goose
	adapters["aider"] = aider
	adapters["kimi"] = kimi
	adapters["mistral-vibe"] = vibe
	adapters["copilot"] = envOnly("copilot", []string{OpenAI, OpenAIResponses, Anthropic}, func(api API) [][2]string {
		typ, wire := "openai", ""
		switch api.Protocol {
		case Anthropic:
			typ = "anthropic"
		case OpenAIResponses:
			wire = "responses"
		}
		return [][2]string{{"COPILOT_PROVIDER_BASE_URL", api.BaseURL}, {"COPILOT_PROVIDER_TYPE", typ}, {"COPILOT_PROVIDER_API_KEY", api.APIKey},
			{"COPILOT_PROVIDER_WIRE_API", wire}, {"COPILOT_MODEL", api.Model}}
	}, "GitHub Copilot takes it from the environment: the sessions enowX starts of it get it.")
	adapters["openclaude"] = envOnly("openclaude", []string{OpenAI, Anthropic}, func(api API) [][2]string {
		if api.Protocol == Anthropic {
			return [][2]string{{"ANTHROPIC_BASE_URL", api.BaseURL}, {"ANTHROPIC_AUTH_TOKEN", api.APIKey}, {"ANTHROPIC_MODEL", api.Model}}
		}
		return [][2]string{{"CLAUDE_CODE_USE_OPENAI", "1"}, {"OPENAI_BASE_URL", api.BaseURL}, {"OPENAI_API_KEY", api.APIKey}, {"OPENAI_MODEL", api.Model}}
	}, "OpenClaude takes it from the environment: the sessions enowX starts of it get it.")
	adapters["continue"] = continueCLI
	adapters["pi"] = pi
}

// The variables enowX keeps for an agent's sessions (envFile).

func readEnv(home, id string) (string, error) {
	data, err := readFile(home, envFile(id))
	return string(data), err
}

func setEnv(home, id string, values [][2]string) error {
	text, err := readEnv(home, id)
	if err != nil {
		return err
	}
	return writeFile(home, envFile(id), []byte(envSet(text, values)))
}

func clearEnv(home, id string, keys ...string) error {
	var values [][2]string
	for _, k := range keys {
		values = append(values, [2]string{k, ""})
	}
	return setEnv(home, id, values)
}

func noModel(a adapter) adapter {
	a.noModel = true
	return a
}

// protocolVar records, in an agent's variables, the protocol they are of;
// sessions don't get it.
const protocolVar = "ENOWX_API_PROTOCOL"

// envOnly is an agent that takes its API from the environment only: the
// variables go to the sessions enowX starts of it.
func envOnly(id string, protocols []string, vars func(API) [][2]string, note string) adapter {
	keys := func() []string {
		out := []string{protocolVar}
		for _, p := range protocols {
			for _, kv := range vars(API{Protocol: p}) {
				out = append(out, kv[0])
			}
		}
		return out
	}
	return adapter{
		protocols: protocols,
		files:     []string{envFile(id)},
		note:      note,
		read: func(home string) (API, bool, error) {
			text, err := readEnv(home, id)
			if err != nil {
				return API{}, false, err
			}
			p := envValue(text, protocolVar)
			if p == "" {
				return API{}, false, nil
			}
			// Which variable holds what: those given these markers.
			api := API{Protocol: p}
			for _, kv := range vars(API{Protocol: p, BaseURL: "\x01url", Model: "\x01model", APIKey: "\x01key"}) {
				v := envValue(text, kv[0])
				switch kv[1] {
				case "\x01url":
					api.BaseURL = v
				case "\x01model":
					api.Model = v
				case "\x01key":
					api.APIKey = v
				}
			}
			return api, api.BaseURL != "", nil
		},
		write: func(home string, api API) error {
			if err := clearEnv(home, id, keys()...); err != nil {
				return err
			}
			return setEnv(home, id, append(vars(api), [2]string{protocolVar, api.Protocol}))
		},
		clear: func(home string) error { return clearEnv(home, id, keys()...) },
	}
}

func removeFile(home, rel string) error {
	err := os.Remove(filepath.Join(home, rel))
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	return err
}

func yamlQuote(s string) string { return strconv.Quote(s) }

// Claude Code: its settings' env block, which it applies itself.
var claudeCode = adapter{
	protocols: []string{Anthropic},
	files:     []string{".claude/settings.json"},
	small:     true,
	note:      "A key set here wins over the claude.ai sign-in, which stays saved for when it is cleared.",
	read: func(home string) (API, bool, error) {
		f := ".claude/settings.json"
		url, err := jsonString(home, f, "env", "ANTHROPIC_BASE_URL")
		if err != nil || url == "" {
			return API{}, false, err
		}
		key, _ := jsonString(home, f, "env", "ANTHROPIC_AUTH_TOKEN")
		if key == "" {
			key, _ = jsonString(home, f, "env", "ANTHROPIC_API_KEY")
		}
		model, _ := jsonString(home, f, "env", "ANTHROPIC_MODEL")
		small, _ := jsonString(home, f, "env", "ANTHROPIC_DEFAULT_HAIKU_MODEL")
		return API{BaseURL: url, APIKey: key, Model: model, SmallModel: small, Protocol: Anthropic}, true, nil
	},
	write: func(home string, api API) error {
		set := func(k, v string) jsonEdit {
			if v == "" {
				return jsonEdit{path: []string{"env", k}}
			}
			return jsonEdit{path: []string{"env", k}, value: v}
		}
		return editJSON(home, ".claude/settings.json",
			set("ANTHROPIC_BASE_URL", api.BaseURL), set("ANTHROPIC_AUTH_TOKEN", api.APIKey), jsonEdit{path: []string{"env", "ANTHROPIC_API_KEY"}},
			set("ANTHROPIC_MODEL", api.Model), set("ANTHROPIC_DEFAULT_HAIKU_MODEL", api.SmallModel))
	},
	clear: func(home string) error {
		var edits []jsonEdit
		for _, k := range []string{"ANTHROPIC_BASE_URL", "ANTHROPIC_AUTH_TOKEN", "ANTHROPIC_API_KEY", "ANTHROPIC_MODEL", "ANTHROPIC_DEFAULT_HAIKU_MODEL"} {
			edits = append(edits, jsonEdit{path: []string{"env", k}})
		}
		return editJSON(home, ".claude/settings.json", edits...)
	},
}

// Codex: a provider of its config, its key from a variable the sessions
// enowX starts get (Codex reads keys from the environment only).
const codexKeyVar = "ENOWX_CODEX_API_KEY"

var codex = adapter{
	protocols: []string{OpenAIResponses},
	files:     []string{".codex/config.toml", envFile("codex")},
	note:      "Codex speaks OpenAI's Responses API only: the API must serve /responses. Its key reaches the sessions enowX starts.",
	read: func(home string) (API, bool, error) {
		data, err := readFile(home, ".codex/config.toml")
		if err != nil {
			return API{}, false, err
		}
		text := string(data)
		if tomlTop(text, "model_provider") != ourName {
			return API{}, false, nil
		}
		p := tomlTable(text, "model_providers."+ourName)
		env, _ := readEnv(home, "codex")
		return API{BaseURL: p["base_url"], Model: tomlTop(text, "model"), APIKey: envValue(env, codexKeyVar), Protocol: OpenAIResponses}, true, nil
	},
	write: func(home string, api API) error {
		data, err := readFile(home, ".codex/config.toml")
		if err != nil {
			return err
		}
		text := tomlSetTop(string(data), [][2]string{{"model_provider", ourName}, {"model", api.Model}})
		text = tomlSetTable(text, "model_providers."+ourName, fmt.Sprintf("name = %s\nbase_url = %s\nenv_key = %s\nwire_api = \"responses\"",
			tomlQuote("enowX: "+api.BaseURL), tomlQuote(api.BaseURL), tomlQuote(codexKeyVar)))
		if err := writeFile(home, ".codex/config.toml", []byte(text)); err != nil {
			return err
		}
		return setEnv(home, "codex", [][2]string{{codexKeyVar, api.APIKey}})
	},
	clear: func(home string) error {
		data, err := readFile(home, ".codex/config.toml")
		if err != nil {
			return err
		}
		text := string(data)
		values := [][2]string{{"model_provider", ""}}
		if tomlTop(text, "model_provider") == ourName {
			values = append(values, [2]string{"model", ""}) // the model was ours
		}
		text = tomlSetTable(tomlSetTop(text, values), "model_providers."+ourName, "")
		if err := writeFile(home, ".codex/config.toml", []byte(text)); err != nil {
			return err
		}
		return clearEnv(home, "codex", codexKeyVar)
	},
}

// OpenCode, and Kilo, which shares its configuration: a provider of
// theirs, by the AI SDK package that speaks the protocol.
func openCodeLike(id, file string) adapter {
	npm := map[string]string{OpenAI: "@ai-sdk/openai-compatible", OpenAIResponses: "@ai-sdk/openai", Anthropic: "@ai-sdk/anthropic"}
	return adapter{
		protocols: []string{OpenAI, OpenAIResponses, Anthropic},
		files:     []string{file},
		note:      "It needs the model's name.",
		read: func(home string) (API, bool, error) {
			p, err := jsonAny(home, file, "provider", ourName)
			if err != nil || p == nil {
				return API{}, false, err
			}
			pm, _ := p.(map[string]any)
			opts, _ := pm["options"].(map[string]any)
			api := API{}
			api.BaseURL, _ = opts["baseURL"].(string)
			api.APIKey, _ = opts["apiKey"].(string)
			pkg, _ := pm["npm"].(string)
			for proto, n := range npm {
				if n == pkg {
					api.Protocol = proto
				}
			}
			model, _ := jsonString(home, file, "model")
			api.Model, _ = strings.CutPrefix(model, ourName+"/")
			return api, true, nil
		},
		write: func(home string, api API) error {
			if api.Model == "" {
				return fmt.Errorf("agentapi: %s needs the model's name", id)
			}
			provider := map[string]any{
				"npm":     npm[api.Protocol],
				"name":    "enowX: " + api.BaseURL,
				"options": map[string]any{"baseURL": api.BaseURL, "apiKey": api.APIKey},
				"models":  map[string]any{api.Model: map[string]any{"name": api.Model}},
			}
			return editJSON(home, file, jsonEdit{path: []string{"provider", ourName}, value: provider}, jsonEdit{path: []string{"model"}, value: ourName + "/" + api.Model})
		},
		clear: func(home string) error {
			edits := []jsonEdit{{path: []string{"provider", ourName}}}
			if m, _ := jsonString(home, file, "model"); strings.HasPrefix(m, ourName+"/") {
				edits = append(edits, jsonEdit{path: []string{"model"}})
			}
			return editJSON(home, file, edits...)
		},
	}
}

// Gemini CLI: its .env for the key and the address, its settings for the
// API key sign-in and the model.
var geminiCLI = adapter{
	protocols: []string{Gemini},
	files:     []string{".gemini/.env", ".gemini/settings.json"},
	note:      "Gemini CLI speaks Gemini's API only; it signs in with the key instead of a Google account.",
	read: func(home string) (API, bool, error) {
		data, err := readFile(home, ".gemini/.env")
		if err != nil {
			return API{}, false, err
		}
		url := envValue(string(data), "GOOGLE_GEMINI_BASE_URL")
		if url == "" {
			return API{}, false, nil
		}
		model, _ := jsonString(home, ".gemini/settings.json", "model", "name")
		return API{BaseURL: url, APIKey: envValue(string(data), "GEMINI_API_KEY"), Model: model, Protocol: Gemini}, true, nil
	},
	write: func(home string, api API) error {
		data, err := readFile(home, ".gemini/.env")
		if err != nil {
			return err
		}
		text := envSet(string(data), [][2]string{{"GOOGLE_GEMINI_BASE_URL", api.BaseURL}, {"GEMINI_API_KEY", api.APIKey}})
		if err := writeFile(home, ".gemini/.env", []byte(text)); err != nil {
			return err
		}
		edits := []jsonEdit{{path: []string{"security", "auth", "selectedType"}, value: "gemini-api-key"}}
		if api.Model != "" {
			edits = append(edits, jsonEdit{path: []string{"model", "name"}, value: api.Model})
		}
		return editJSON(home, ".gemini/settings.json", edits...)
	},
	clear: func(home string) error {
		data, err := readFile(home, ".gemini/.env")
		if err != nil {
			return err
		}
		return writeFile(home, ".gemini/.env", []byte(envSet(string(data), [][2]string{{"GOOGLE_GEMINI_BASE_URL", ""}, {"GEMINI_API_KEY", ""}})))
	},
}

// Qwen Code: a model provider of its settings, its key in the settings'
// env block under a name of enowX's.
const qwenKeyVar = "ENOWX_QWEN_API_KEY"

var qwenCode = adapter{
	protocols: []string{OpenAI, OpenAIResponses, Anthropic},
	files:     []string{".qwen/settings.json"},
	note:      "It needs the model's name.",
	read: func(home string) (API, bool, error) {
		f := ".qwen/settings.json"
		for _, kind := range []string{"openai", "anthropic"} {
			list, err := jsonAny(home, f, "modelProviders", kind)
			if err != nil {
				return API{}, false, err
			}
			items, _ := list.([]any)
			for _, it := range items {
				m, _ := it.(map[string]any)
				if m["envKey"] != qwenKeyVar {
					continue
				}
				api := API{Protocol: OpenAI}
				api.BaseURL, _ = m["baseUrl"].(string)
				api.Model, _ = m["id"].(string)
				api.APIKey, _ = jsonString(home, f, "env", qwenKeyVar)
				if kind == "anthropic" {
					api.Protocol = Anthropic
				} else if m["wireApi"] == "responses" {
					api.Protocol = OpenAIResponses
				}
				return api, true, nil
			}
		}
		return API{}, false, nil
	},
	write: func(home string, api API) error {
		if api.Model == "" {
			return fmt.Errorf("agentapi: Qwen Code needs the model's name")
		}
		if err := qwenRemove(home); err != nil {
			return err
		}
		kind := "openai"
		entry := map[string]any{"id": api.Model, "envKey": qwenKeyVar, "baseUrl": api.BaseURL}
		switch api.Protocol {
		case Anthropic:
			kind = "anthropic"
		case OpenAIResponses:
			entry["wireApi"] = "responses"
		}
		list, err := jsonAny(home, ".qwen/settings.json", "modelProviders", kind)
		if err != nil {
			return err
		}
		items, _ := list.([]any)
		items = append(items, entry)
		return editJSON(home, ".qwen/settings.json",
			jsonEdit{path: []string{"modelProviders", kind}, value: items},
			jsonEdit{path: []string{"env", qwenKeyVar}, value: api.APIKey},
			jsonEdit{path: []string{"security", "auth", "selectedType"}, value: kind},
			jsonEdit{path: []string{"model", "name"}, value: api.Model})
	},
	clear: func(home string) error {
		if err := qwenRemove(home); err != nil {
			return err
		}
		return editJSON(home, ".qwen/settings.json", jsonEdit{path: []string{"env", qwenKeyVar}})
	},
}

// qwenRemove takes enowX's providers out of Qwen Code's settings.
func qwenRemove(home string) error {
	var edits []jsonEdit
	for _, kind := range []string{"openai", "anthropic"} {
		list, err := jsonAny(home, ".qwen/settings.json", "modelProviders", kind)
		if err != nil {
			return err
		}
		items, ok := list.([]any)
		if !ok {
			continue
		}
		var kept []any
		for _, it := range items {
			if m, _ := it.(map[string]any); m["envKey"] != qwenKeyVar {
				kept = append(kept, it)
			}
		}
		if len(kept) == 0 {
			edits = append(edits, jsonEdit{path: []string{"modelProviders", kind}})
		} else {
			edits = append(edits, jsonEdit{path: []string{"modelProviders", kind}, value: kept})
		}
	}
	if len(edits) == 0 {
		return nil
	}
	return editJSON(home, ".qwen/settings.json", edits...)
}

// Droid: a custom model of its settings, which /model then offers.
var droid = adapter{
	protocols: []string{OpenAI, OpenAIResponses, Anthropic},
	files:     []string{".factory/settings.json"},
	note:      "It shows in Droid's /model as \"enowX: <model>\"; pick it there. It needs the model's name.",
	read: func(home string) (API, bool, error) {
		list, err := jsonAny(home, ".factory/settings.json", "customModels")
		if err != nil {
			return API{}, false, err
		}
		items, _ := list.([]any)
		for _, it := range items {
			m, _ := it.(map[string]any)
			if name, _ := m["displayName"].(string); !strings.HasPrefix(name, "enowX: ") {
				continue
			}
			api := API{}
			api.BaseURL, _ = m["baseUrl"].(string)
			api.APIKey, _ = m["apiKey"].(string)
			api.Model, _ = m["model"].(string)
			switch m["provider"] {
			case "anthropic":
				api.Protocol = Anthropic
			case "openai":
				api.Protocol = OpenAIResponses
			default:
				api.Protocol = OpenAI
			}
			return api, true, nil
		}
		return API{}, false, nil
	},
	write: func(home string, api API) error {
		if api.Model == "" {
			return fmt.Errorf("agentapi: Droid needs the model's name")
		}
		items, err := droidOthers(home)
		if err != nil {
			return err
		}
		provider := map[string]string{OpenAI: "generic-chat-completion-api", OpenAIResponses: "openai", Anthropic: "anthropic"}[api.Protocol]
		items = append(items, map[string]any{"model": api.Model, "displayName": "enowX: " + api.Model, "baseUrl": api.BaseURL, "apiKey": api.APIKey, "provider": provider, "maxOutputTokens": 16384})
		return editJSON(home, ".factory/settings.json", jsonEdit{path: []string{"customModels"}, value: items})
	},
	clear: func(home string) error {
		items, err := droidOthers(home)
		if err != nil {
			return err
		}
		if len(items) == 0 {
			return editJSON(home, ".factory/settings.json", jsonEdit{path: []string{"customModels"}})
		}
		return editJSON(home, ".factory/settings.json", jsonEdit{path: []string{"customModels"}, value: items})
	},
}

// droidOthers are Droid's custom models but enowX's.
func droidOthers(home string) ([]any, error) {
	list, err := jsonAny(home, ".factory/settings.json", "customModels")
	if err != nil {
		return nil, err
	}
	items, _ := list.([]any)
	var out []any
	for _, it := range items {
		if m, _ := it.(map[string]any); !strings.HasPrefix(fmt.Sprint(m["displayName"]), "enowX: ") {
			out = append(out, it)
		}
	}
	return out, nil
}

// Goose: a custom provider file of enowX's, chosen in its config, its key
// from a variable the sessions enowX starts get (Goose ignores keys in
// its config).
const gooseKeyVar = "ENOWX_GOOSE_API_KEY"

var goose = adapter{
	protocols: []string{OpenAI, Anthropic},
	files:     []string{".config/goose/config.yaml", ".config/goose/custom_providers/enowx.json", envFile("goose")},
	note:      "It needs the model's name. Its key reaches the sessions enowX starts.",
	read: func(home string) (API, bool, error) {
		data, err := readFile(home, ".config/goose/config.yaml")
		if err != nil || yamlTop(string(data), "GOOSE_PROVIDER") != ourName {
			return API{}, false, err
		}
		f := ".config/goose/custom_providers/enowx.json"
		url, _ := jsonString(home, f, "base_url")
		engine, _ := jsonString(home, f, "engine")
		env, _ := readEnv(home, "goose")
		api := API{BaseURL: strings.TrimSuffix(url, "/chat/completions"), Model: yamlTop(string(data), "GOOSE_MODEL"), APIKey: envValue(env, gooseKeyVar), Protocol: OpenAI}
		if engine == "anthropic" {
			api.Protocol = Anthropic
		}
		return api, true, nil
	},
	write: func(home string, api API) error {
		if api.Model == "" {
			return fmt.Errorf("agentapi: Goose needs the model's name")
		}
		engine, url := "openai", api.BaseURL
		if api.Protocol == Anthropic {
			engine = "anthropic"
		} else if !strings.HasSuffix(url, "/chat/completions") {
			url += "/chat/completions" // Goose's openai engine takes the endpoint itself
		}
		if err := editJSON(home, ".config/goose/custom_providers/enowx.json",
			jsonEdit{path: []string{"name"}, value: ourName}, jsonEdit{path: []string{"display_name"}, value: "enowX: " + api.BaseURL},
			jsonEdit{path: []string{"engine"}, value: engine}, jsonEdit{path: []string{"api_key_env"}, value: gooseKeyVar},
			jsonEdit{path: []string{"base_url"}, value: url},
			jsonEdit{path: []string{"models"}, value: []any{map[string]any{"name": api.Model, "context_limit": 128000}}}); err != nil {
			return err
		}
		data, err := readFile(home, ".config/goose/config.yaml")
		if err != nil {
			return err
		}
		if err := writeFile(home, ".config/goose/config.yaml", []byte(yamlSetTop(string(data), [][2]string{{"GOOSE_PROVIDER", ourName}, {"GOOSE_MODEL", api.Model}}))); err != nil {
			return err
		}
		return setEnv(home, "goose", [][2]string{{gooseKeyVar, api.APIKey}})
	},
	clear: func(home string) error {
		data, err := readFile(home, ".config/goose/config.yaml")
		if err != nil {
			return err
		}
		if yamlTop(string(data), "GOOSE_PROVIDER") == ourName {
			data = []byte(yamlSetTop(string(data), [][2]string{{"GOOSE_PROVIDER", ""}, {"GOOSE_MODEL", ""}}))
			if err := writeFile(home, ".config/goose/config.yaml", data); err != nil {
				return err
			}
		}
		if err := removeFile(home, ".config/goose/custom_providers/enowx.json"); err != nil {
			return err
		}
		return clearEnv(home, "goose", gooseKeyVar)
	},
}

// Aider: its home config's OpenAI-compatible address, key and model.
var aider = adapter{
	protocols: []string{OpenAI},
	files:     []string{".aider.conf.yml"},
	note:      "It needs the model's name. A project's own .aider.conf.yml wins over this one.",
	read: func(home string) (API, bool, error) {
		data, err := readFile(home, ".aider.conf.yml")
		if err != nil {
			return API{}, false, err
		}
		text := string(data)
		url := yamlTop(text, "openai-api-base")
		if url == "" {
			return API{}, false, nil
		}
		model, _ := strings.CutPrefix(yamlTop(text, "model"), "openai/")
		return API{BaseURL: url, APIKey: yamlTop(text, "openai-api-key"), Model: model, Protocol: OpenAI}, true, nil
	},
	write: func(home string, api API) error {
		if api.Model == "" {
			return fmt.Errorf("agentapi: Aider needs the model's name")
		}
		data, err := readFile(home, ".aider.conf.yml")
		if err != nil {
			return err
		}
		text := yamlSetTop(string(data), [][2]string{{"openai-api-base", api.BaseURL}, {"openai-api-key", api.APIKey}, {"model", "openai/" + api.Model}})
		return writeFile(home, ".aider.conf.yml", []byte(text))
	},
	clear: func(home string) error {
		data, err := readFile(home, ".aider.conf.yml")
		if err != nil {
			return err
		}
		text := string(data)
		values := [][2]string{{"openai-api-base", ""}, {"openai-api-key", ""}}
		if strings.HasPrefix(yamlTop(text, "model"), "openai/") {
			values = append(values, [2]string{"model", ""})
		}
		return writeFile(home, ".aider.conf.yml", []byte(yamlSetTop(text, values)))
	},
}

// Kimi: a provider and a model of its config, the model its default.
var kimi = adapter{
	protocols: []string{OpenAI, OpenAIResponses, Anthropic, Gemini},
	files:     []string{".kimi/config.toml"},
	note:      "It needs the model's name.",
	read: func(home string) (API, bool, error) {
		data, err := readFile(home, ".kimi/config.toml")
		if err != nil {
			return API{}, false, err
		}
		text := string(data)
		p := tomlTable(text, "providers."+ourName)
		if p == nil || tomlTop(text, "default_model") != ourName {
			return API{}, false, nil
		}
		m := tomlTable(text, "models."+ourName)
		api := API{BaseURL: p["base_url"], APIKey: p["api_key"], Model: m["model"]}
		for proto, typ := range kimiTypes {
			if typ == p["type"] {
				api.Protocol = proto
			}
		}
		return api, true, nil
	},
	write: func(home string, api API) error {
		if api.Model == "" {
			return fmt.Errorf("agentapi: Kimi needs the model's name")
		}
		data, err := readFile(home, ".kimi/config.toml")
		if err != nil {
			return err
		}
		text := tomlSetTop(string(data), [][2]string{{"default_model", ourName}})
		text = tomlSetTable(text, "providers."+ourName, fmt.Sprintf("type = %s\nbase_url = %s\napi_key = %s", tomlQuote(kimiTypes[api.Protocol]), tomlQuote(api.BaseURL), tomlQuote(api.APIKey)))
		text = tomlSetTable(text, "models."+ourName, fmt.Sprintf("provider = %s\nmodel = %s\nmax_context_size = 131072", tomlQuote(ourName), tomlQuote(api.Model)))
		return writeFile(home, ".kimi/config.toml", []byte(text))
	},
	clear: func(home string) error {
		data, err := readFile(home, ".kimi/config.toml")
		if err != nil {
			return err
		}
		text := string(data)
		if tomlTop(text, "default_model") == ourName {
			text = tomlSetTop(text, [][2]string{{"default_model", ""}})
		}
		text = tomlSetTable(tomlSetTable(text, "providers."+ourName, ""), "models."+ourName, "")
		return writeFile(home, ".kimi/config.toml", []byte(text))
	},
}

var kimiTypes = map[string]string{OpenAI: "openai_legacy", OpenAIResponses: "openai_responses", Anthropic: "anthropic", Gemini: "gemini"}

// Mistral Vibe: a provider and a model of its config (arrays of tables,
// enowX's named "enowx"), its key in its .env, which Vibe reads.
const vibeKeyVar = "ENOWX_VIBE_API_KEY"

var vibe = adapter{
	protocols: []string{OpenAI, OpenAIResponses, Anthropic},
	files:     []string{".vibe/config.toml", ".vibe/.env"},
	note:      "It needs the model's name.",
	read: func(home string) (API, bool, error) {
		data, err := readFile(home, ".vibe/config.toml")
		if err != nil {
			return API{}, false, err
		}
		text := string(data)
		if tomlTop(text, "active_model") != ourName {
			return API{}, false, nil
		}
		p := tomlArrayItem(text, "providers", "name", ourName)
		m := tomlArrayItem(text, "models", "alias", ourName)
		env, _ := readFile(home, ".vibe/.env")
		api := API{BaseURL: p["api_base"], Model: m["name"], APIKey: envValue(string(env), vibeKeyVar), Protocol: OpenAI}
		switch p["api_style"] {
		case "openai-responses":
			api.Protocol = OpenAIResponses
		case "anthropic":
			api.Protocol = Anthropic
		}
		return api, true, nil
	},
	write: func(home string, api API) error {
		if api.Model == "" {
			return fmt.Errorf("agentapi: Mistral Vibe needs the model's name")
		}
		data, err := readFile(home, ".vibe/config.toml")
		if err != nil {
			return err
		}
		text := tomlRemoveArrayItem(string(data), "providers", "name", ourName)
		text = tomlRemoveArrayItem(text, "models", "alias", ourName)
		text = tomlSetTop(text, [][2]string{{"active_model", ourName}})
		text = strings.TrimRight(text, "\n") + fmt.Sprintf("\n\n[[providers]]\nname = %s\napi_base = %s\napi_key_env_var = %s\napi_style = %s\nbackend = \"generic\"\n\n[[models]]\nname = %s\nprovider = %s\nalias = %s\n",
			tomlQuote(ourName), tomlQuote(api.BaseURL), tomlQuote(vibeKeyVar), tomlQuote(api.Protocol), tomlQuote(api.Model), tomlQuote(ourName), tomlQuote(ourName))
		if err := writeFile(home, ".vibe/config.toml", []byte(text)); err != nil {
			return err
		}
		env, err := readFile(home, ".vibe/.env")
		if err != nil {
			return err
		}
		return writeFile(home, ".vibe/.env", []byte(envSet(string(env), [][2]string{{vibeKeyVar, api.APIKey}})))
	},
	clear: func(home string) error {
		data, err := readFile(home, ".vibe/config.toml")
		if err != nil {
			return err
		}
		text := tomlRemoveArrayItem(tomlRemoveArrayItem(string(data), "providers", "name", ourName), "models", "alias", ourName)
		if tomlTop(text, "active_model") == ourName {
			text = tomlSetTop(text, [][2]string{{"active_model", ""}})
		}
		if err := writeFile(home, ".vibe/config.toml", []byte(text)); err != nil {
			return err
		}
		env, err := readFile(home, ".vibe/.env")
		if err != nil {
			return err
		}
		return writeFile(home, ".vibe/.env", []byte(envSet(string(env), [][2]string{{vibeKeyVar, ""}})))
	},
}

// Continue: a config of enowX's, which the sessions enowX starts of it
// are given (--config), the user's own left as it is.
var continueFile = filepath.Join(".enowx-app", "agentapi", "continue.yaml")

var continueCLI = adapter{
	protocols: []string{OpenAI, Anthropic, Gemini},
	files:     []string{continueFile},
	note:      "It needs the model's name. The sessions enowX starts of it use it; your own config stays as it is.",
	args: func(home string) []string {
		if data, _ := readFile(home, continueFile); len(data) > 0 {
			return []string{"--config", filepath.Join(home, continueFile)}
		}
		return nil
	},
	read: func(home string) (API, bool, error) {
		data, err := readFile(home, continueFile)
		if err != nil || len(data) == 0 {
			return API{}, false, err
		}
		text := string(data)
		api := API{Protocol: OpenAI}
		for _, l := range strings.Split(text, "\n") {
			k, v, ok := strings.Cut(strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(l), "- ")), ":")
			if !ok {
				continue
			}
			v = strings.Trim(strings.TrimSpace(v), `"`)
			switch k {
			case "provider":
				api.Protocol = v
			case "model":
				api.Model = v
			case "apiBase":
				api.BaseURL = v
			case "apiKey":
				api.APIKey = v
			}
		}
		return api, api.BaseURL != "", nil
	},
	write: func(home string, api API) error {
		if api.Model == "" {
			return fmt.Errorf("agentapi: Continue needs the model's name")
		}
		text := fmt.Sprintf("name: enowX\nversion: 0.0.1\nschema: v1\nmodels:\n  - name: %s\n    provider: %s\n    model: %s\n    apiBase: %s\n    apiKey: %s\n    roles: [chat, edit, apply]\n",
			yamlQuote(api.Model), api.Protocol, yamlQuote(api.Model), yamlQuote(api.BaseURL), yamlQuote(api.APIKey))
		return writeFile(home, continueFile, []byte(text))
	},
	clear: func(home string) error { return removeFile(home, continueFile) },
}

// Pi: a provider of its models, its settings' default.
var pi = adapter{
	protocols: []string{OpenAI, OpenAIResponses, Anthropic, Gemini},
	files:     []string{".pi/agent/models.json", ".pi/agent/settings.json"},
	note:      "It needs the model's name.",
	read: func(home string) (API, bool, error) {
		p, err := jsonAny(home, ".pi/agent/models.json", "providers", ourName)
		if err != nil || p == nil {
			return API{}, false, err
		}
		if def, _ := jsonString(home, ".pi/agent/settings.json", "defaultProvider"); def != ourName {
			return API{}, false, nil
		}
		m, _ := p.(map[string]any)
		api := API{}
		api.BaseURL, _ = m["baseUrl"].(string)
		api.APIKey, _ = m["apiKey"].(string)
		api.Model, _ = jsonString(home, ".pi/agent/settings.json", "defaultModel")
		for proto, name := range piAPIs {
			if name == m["api"] {
				api.Protocol = proto
			}
		}
		return api, true, nil
	},
	write: func(home string, api API) error {
		if api.Model == "" {
			return fmt.Errorf("agentapi: Pi needs the model's name")
		}
		provider := map[string]any{"baseUrl": api.BaseURL, "api": piAPIs[api.Protocol], "apiKey": api.APIKey, "models": []any{map[string]any{"id": api.Model}}}
		if err := editJSON(home, ".pi/agent/models.json", jsonEdit{path: []string{"providers", ourName}, value: provider}); err != nil {
			return err
		}
		return editJSON(home, ".pi/agent/settings.json", jsonEdit{path: []string{"defaultProvider"}, value: ourName}, jsonEdit{path: []string{"defaultModel"}, value: api.Model})
	},
	clear: func(home string) error {
		if err := editJSON(home, ".pi/agent/models.json", jsonEdit{path: []string{"providers", ourName}}); err != nil {
			return err
		}
		if def, _ := jsonString(home, ".pi/agent/settings.json", "defaultProvider"); def == ourName {
			return editJSON(home, ".pi/agent/settings.json", jsonEdit{path: []string{"defaultProvider"}}, jsonEdit{path: []string{"defaultModel"}})
		}
		return nil
	},
}

var piAPIs = map[string]string{OpenAI: "openai-completions", OpenAIResponses: "openai-responses", Anthropic: "anthropic-messages", Gemini: "google-generative-ai"}
