package daemon

import (
	"fmt"
	"os"
	"path/filepath"

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
	proto.MethodEnvProjectScan: with(func(c *conn, p proto.EnvProjectRef) (any, error) {
		dir, err := projectDir(p.Project)
		if err != nil {
			return nil, err
		}
		return envsync.ScanIn(dir, envsync.Project), nil
	}),
	proto.MethodEnvProjectExport: with(func(c *conn, p proto.EnvProjectRef) (any, error) {
		dir, err := projectDir(p.Project)
		if err != nil {
			return nil, err
		}
		return envsync.ExportIn(dir, p.ID, envsync.Project)
	}),
	proto.MethodEnvProjectImport: with(func(c *conn, p proto.EnvProjectImport) (any, error) {
		dir, err := projectDir(p.Project)
		if err != nil {
			return nil, err
		}
		return envsync.ImportIn(dir, p.Payload, envsync.Project)
	}),
	proto.MethodEnvProjectDelete: with(func(c *conn, p proto.EnvProjectRef) (any, error) {
		dir, err := projectDir(p.Project)
		if err != nil {
			return nil, err
		}
		return ok, envsync.DeleteIn(dir, p.ID, envsync.Project)
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

// projectDir is a project's folder, which must be one here.
func projectDir(p string) (string, error) {
	if info, err := os.Stat(p); err != nil || !info.IsDir() || !filepath.IsAbs(p) {
		return "", fmt.Errorf("%s isn't a project folder here", p)
	}
	return p, nil
}
