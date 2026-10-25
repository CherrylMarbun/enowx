package canvas

import (
	"fmt"
	"strings"
)

// Component is a piece of UI ready to put in a mockup: its name, and its
// HTML with ID where each copy's id goes.
type Component struct {
	Name string
	HTML string
}

// Components are the pieces the Canvas page inserts, styled with the
// canvas guide's default tokens.
var Components = []Component{
	{"Navbar", `<header id="ID" style="display:flex;align-items:center;gap:24px;height:64px;padding:0 32px;border-bottom:1px solid #e2e8f0;background:#fff;font:14px Inter,system-ui,sans-serif;color:#0f172a"><strong>Acme</strong><nav style="display:flex;gap:20px;color:#475569"><a>Product</a><a>Pricing</a><a>Docs</a></nav><span style="flex:1"></span><button style="height:36px;padding:0 14px;border-radius:8px;border:0;background:#2563eb;color:#fff;font:500 14px Inter,system-ui">Start free</button></header>`},
	{"Hero", `<section id="ID" style="padding:72px 32px;text-align:center;font-family:Inter,system-ui,sans-serif;color:#0f172a"><h1 style="font-size:40px;line-height:1.2;margin:0 0 12px">Ship faster, together</h1><p style="font-size:18px;color:#64748b;margin:0 0 24px">One place for your team's work.</p><button style="height:44px;padding:0 20px;border-radius:8px;border:0;background:#2563eb;color:#fff;font:500 15px Inter,system-ui">Get started</button></section>`},
	{"Card", `<article id="ID" style="background:#fff;border:1px solid #e2e8f0;border-radius:12px;padding:20px;display:grid;gap:8px;max-width:320px;font-family:Inter,system-ui,sans-serif"><span style="color:#64748b;font-size:13px">Revenue</span><strong style="font-size:28px;color:#0f172a">$48,210</strong><span style="color:#16a34a;font-size:13px">+12% this month</span></article>`},
	{"Form", `<form id="ID" style="display:grid;gap:14px;max-width:380px;padding:24px;font-family:Inter,system-ui,sans-serif;color:#0f172a"><label style="display:grid;gap:6px;font-size:14px;font-weight:500">Email<input type="email" placeholder="ada@example.com" style="height:40px;padding:0 12px;border:1px solid #e2e8f0;border-radius:8px;font:14px Inter,system-ui"></label><label style="display:grid;gap:6px;font-size:14px;font-weight:500">Password<input type="password" style="height:40px;padding:0 12px;border:1px solid #e2e8f0;border-radius:8px"></label><button style="height:40px;border-radius:8px;border:0;background:#2563eb;color:#fff;font:500 14px Inter,system-ui">Sign in</button></form>`},
	{"Table", `<table id="ID" style="width:100%;border-collapse:collapse;font:14px Inter,system-ui,sans-serif;color:#0f172a"><thead><tr style="text-align:left;color:#64748b;border-bottom:1px solid #e2e8f0"><th style="padding:10px 12px">Order</th><th style="padding:10px 12px">Customer</th><th style="padding:10px 12px">Total</th></tr></thead><tbody><tr style="border-bottom:1px solid #f1f5f9"><td style="padding:10px 12px">#1042</td><td style="padding:10px 12px">Ada Lovelace</td><td style="padding:10px 12px">$120.00</td></tr><tr><td style="padding:10px 12px">#1041</td><td style="padding:10px 12px">Alan Turing</td><td style="padding:10px 12px">$64.50</td></tr></tbody></table>`},
	{"Modal", `<div id="ID" style="position:fixed;inset:0;background:rgba(15,23,42,.4);display:grid;place-items:center;font-family:Inter,system-ui,sans-serif"><div role="dialog" style="width:440px;background:#fff;border-radius:12px;padding:24px;display:grid;gap:16px;color:#0f172a"><strong style="font-size:18px">Delete project?</strong><p style="margin:0;color:#64748b;font-size:14px">This can't be undone.</p><div style="display:flex;justify-content:flex-end;gap:8px"><button style="height:36px;padding:0 14px;border-radius:8px;border:1px solid #e2e8f0;background:#fff">Cancel</button><button style="height:36px;padding:0 14px;border-radius:8px;border:0;background:#dc2626;color:#fff">Delete</button></div></div></div>`},
}

// ComponentHTML is component c's HTML with an id not yet in page.
func ComponentHTML(c Component, page string) (id, html string) {
	base := strings.ToLower(c.Name)
	taken := map[string]bool{}
	for _, el := range Outline(page) {
		taken[el.ID] = true
	}
	id = base
	for n := 2; taken[id]; n++ {
		id = fmt.Sprintf("%s-%d", base, n)
	}
	return id, strings.Replace(c.HTML, `id="ID"`, `id="`+id+`"`, 1)
}
