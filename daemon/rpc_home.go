package daemon

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/enowx/enowx/core/envsync"
	"github.com/enowx/enowx/core/folders"
	"github.com/enowx/enowx/core/mcptest"
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
	proto.MethodEnvSkillFetch: with(func(c *conn, p proto.SkillFetchParams) (any, error) {
		return c.srv.fetchSkills(p)
	}),
	proto.MethodEnvSkillInstall: with(func(c *conn, p proto.SkillInstallParams) (any, error) {
		return c.srv.installSkills(p)
	}),
	proto.MethodEnvPlugins: inHome0(func(home string) (any, error) { return envsync.ClaudePlugins(home), nil }),
	proto.MethodEnvPluginRun: with(func(c *conn, p proto.PluginRunParams) (any, error) {
		args, err := envsync.PluginArgs(p.Action, p.Arg)
		if err != nil {
			return nil, err
		}
		ctx, cancel := context.WithTimeout(context.Background(), pluginTimeout)
		defer cancel()
		out, err := exec.CommandContext(ctx, "claude", args...).CombinedOutput()
		return proto.PluginRun{OK: err == nil, Output: strings.TrimSpace(string(out))}, nil
	}),
	proto.MethodEnvMCPTest: with(func(c *conn, p proto.EnvProjectRef) (any, error) {
		root, sc := "", envsync.User
		if p.Project != "" {
			dir, err := projectDir(p.Project)
			if err != nil {
				return nil, err
			}
			root, sc = dir, envsync.Project
		} else {
			home, err := os.UserHomeDir()
			if err != nil {
				return nil, err
			}
			root = home
		}
		item, err := envsync.ExportIn(root, p.ID, sc)
		if err != nil {
			return nil, err
		}
		srv, err := envsync.ParseMCP(p.ID, item.Value)
		if err != nil {
			return nil, err
		}
		return mcptest.Test(context.Background(), srv), nil
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

// pluginTimeout bounds Claude Code's CLI changing a plugin: it may clone a
// marketplace.
const pluginTimeout = 3 * time.Minute

// inHome0 adapts a handler that takes the home folder alone.
func inHome0(fn func(home string) (any, error)) handler {
	return bare(func(c *conn) (any, error) {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, err
		}
		return fn(home)
	})
}
