package browserview

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

// Driving a page from the command line (enowx browser): the scripts that
// read it as text with its controls numbered, click one and fill one in.
// The numbers are kept on the elements (data-enowx-ref) until the next
// snapshot.

// Evaluator runs JavaScript in a page and returns its result as JSON
// decodes it.
type Evaluator interface {
	Eval(ctx context.Context, code string) (any, error)
}

// Page returns the view's page, nil before one opened.
func (v *View) Page() Page { return v.page }

// Open loads address (a URL, a host, or a path of the port shown) in the
// view's page, opening one if there is none.
func (v *View) Open(o Options, address string) error {
	v.address = address
	v.navigate(o)
	if v.err != "" {
		return errors.New(v.err)
	}
	return nil
}

// Snapshot is the page as text: what it says, then its controls, each
// with the number to click or fill it by.
func Snapshot(ctx context.Context, page Evaluator) (string, error) {
	var out struct {
		Text     string   `json:"text"`
		Controls []string `json:"controls"`
	}
	if err := evalInto(ctx, page, snapshotScript, &out); err != nil {
		return "", err
	}
	var b strings.Builder
	b.WriteString(strings.TrimSpace(out.Text))
	b.WriteString("\n\nControls:\n")
	if len(out.Controls) == 0 {
		b.WriteString("(none)\n")
	}
	for _, c := range out.Controls {
		b.WriteString(c + "\n")
	}
	return b.String(), nil
}

// Click clicks control ref: a number from Snapshot, or a CSS selector.
func Click(ctx context.Context, page Evaluator, ref string) error {
	return act(ctx, page, "click", ref, "")
}

// Fill types text into control ref, as the page sees typing.
func Fill(ctx context.Context, page Evaluator, ref, text string) error {
	return act(ctx, page, "fill", ref, text)
}

// Title is the page's title.
func Title(ctx context.Context, page Evaluator) string {
	var t string
	_ = evalInto(ctx, page, "document.title", &t)
	return t
}

// Settle waits for page to finish loading what an action started.
func Settle(ctx context.Context, page Page) {
	time.Sleep(300 * time.Millisecond) // what a click starts begins loading
	for page.Loading() && ctx.Err() == nil {
		time.Sleep(100 * time.Millisecond)
	}
}

func act(ctx context.Context, page Evaluator, what, ref, text string) error {
	args, _ := json.Marshal([]string{what, ref, text})
	var out struct {
		Error string `json:"error"`
	}
	if err := evalInto(ctx, page, "("+actScript+")(..."+string(args)+")", &out); err != nil {
		return err
	}
	if out.Error != "" {
		return errors.New(out.Error)
	}
	return nil
}

func evalInto(ctx context.Context, page Evaluator, code string, v any) error {
	raw, err := page.Eval(ctx, code)
	if err != nil {
		return err
	}
	data, err := json.Marshal(raw)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(data, v); err != nil {
		return fmt.Errorf("browser: the page answered %s", data)
	}
	return nil
}

const snapshotScript = `(() => {
  document.querySelectorAll('[data-enowx-ref]').forEach(e => e.removeAttribute('data-enowx-ref'));
  const sel = 'a[href],button,input,textarea,select,summary,[role=button],[role=link],[role=checkbox],[role=radio],[role=tab],[role=menuitem],[role=switch],[contenteditable=""],[contenteditable=true],[onclick]';
  const controls = [];
  let n = 0;
  for (const e of document.querySelectorAll(sel)) {
    const r = e.getBoundingClientRect(), st = getComputedStyle(e);
    if ((r.width === 0 && r.height === 0) || st.visibility === 'hidden' || st.display === 'none' || e.type === 'hidden') continue;
    n++;
    e.setAttribute('data-enowx-ref', String(n));
    const tag = e.tagName.toLowerCase();
    const kind = e.getAttribute('role') || (tag === 'a' ? 'link' : tag === 'input' ? (e.type || 'text') + ' field' : tag);
    const label = e.labels && e.labels[0] ? e.labels[0].innerText : '';
    const name = (e.getAttribute('aria-label') || label || (tag === 'input' || tag === 'textarea' ? '' : e.innerText) || e.placeholder || e.title || e.getAttribute('name') || e.value || '').trim().replace(/\s+/g, ' ').slice(0, 80);
    let line = '[' + n + '] ' + kind + (name ? ' "' + name + '"' : '');
    if (tag === 'a') line += ' -> ' + e.getAttribute('href');
    if ((tag === 'input' || tag === 'textarea') && e.type !== 'password' && e.type !== 'checkbox' && e.type !== 'radio' && e.value) line += ' = "' + e.value.slice(0, 60) + '"';
    if (e.type === 'checkbox' || e.type === 'radio') line += e.checked ? ' (checked)' : ' (not checked)';
    if (tag === 'select' && e.selectedOptions[0]) line += ' = "' + e.selectedOptions[0].text + '"';
    if (e.disabled) line += ' (disabled)';
    controls.push(line);
  }
  const text = (document.body ? document.body.innerText : '').replace(/\n{3,}/g, '\n\n').slice(0, 6000);
  return {text, controls};
})()`

const actScript = `(what, ref, text) => {
  const e = /^\d+$/.test(ref) ? document.querySelector('[data-enowx-ref="' + ref + '"]') : document.querySelector(ref);
  if (!e) return {error: 'no control ' + ref + ' on the page: take a snapshot for the numbers'};
  e.scrollIntoView({block: 'center'});
  if (what === 'click') { e.click(); return {}; }
  e.focus();
  if (e.isContentEditable) {
    e.textContent = text;
  } else if (e.tagName === 'SELECT') {
    const o = Array.from(e.options).find(o => o.value === text || o.text === text);
    if (!o) return {error: 'no option ' + text};
    e.value = o.value;
  } else {
    const proto = e.tagName === 'TEXTAREA' ? HTMLTextAreaElement.prototype : HTMLInputElement.prototype;
    Object.getOwnPropertyDescriptor(proto, 'value').set.call(e, text);
  }
  e.dispatchEvent(new Event('input', {bubbles: true}));
  e.dispatchEvent(new Event('change', {bubbles: true}));
  return {};
}`
