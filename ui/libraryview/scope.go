package libraryview

import (
	"github.com/enowx/enowx/client"
	"github.com/enowx/enowx/proto"
)

// The library shows the user's items, under the home folder of the
// connected host.

func (v *View) fetchScan(cl *client.Client) (proto.EnvScan, error) { return cl.EnvScan() }

func (v *View) fetchItem(cl *client.Client, id string) (proto.EnvPayload, error) {
	return cl.EnvExport(id)
}

func (v *View) writeItem(cl *client.Client, p proto.EnvPayload) (proto.EnvImportResult, error) {
	return cl.EnvImport(p)
}

func (v *View) deleteItem(cl *client.Client, id string) error { return cl.EnvDelete(id) }
