package ui

// togglePage shows a page, or the sessions again if it is showing.
func (a *App) togglePage(id string) {
	if a.page == id {
		a.page = ""
	} else {
		a.page = id
	}
}
