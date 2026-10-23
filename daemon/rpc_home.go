package daemon

import (
	"os"

	"github.com/enowx/enowx/core/envsync"
	"github.com/enowx/enowx/core/folders"
	"github.com/enowx/enowx/proto"
)

// The methods in this file work on the daemon user's home folder.

var folderHandlers = map[string]handler{
	proto.MethodFolderList: inHome(func(home string, p proto.FolderListParams) (any, error) {
		return folders.List(home, p)
	}),
	proto.MethodFolderMake: inHome(func(home string, p proto.FolderRef) (any, error) {
		dir, err := folders.Make(home, p.Path)
		if err != nil {
			return nil, err
		}
		return proto.FolderRef{Path: dir}, nil
	}),
}

var envHandlers = map[string]handler{
	proto.MethodEnvScan: bare(func(c *conn) (any, error) {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, err
		}
		return envsync.Scan(home), nil
	}),
	proto.MethodEnvExport: inHome(func(home string, p proto.EnvItemRef) (any, error) {
		return envsync.Export(home, p.ID)
	}),
	proto.MethodEnvImport: inHome(envsync.Import),
	proto.MethodEnvDelete: inHome(func(home string, p proto.EnvItemRef) (any, error) {
		return ok, envsync.Delete(home, p.ID)
	}),
}

// inHome adapts a handler that takes the home folder and decoded params.
func inHome[P any, R any](fn func(home string, p P) (R, error)) handler {
	return with(func(c *conn, p P) (any, error) {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, err
		}
		return fn(home, p)
	})
}
