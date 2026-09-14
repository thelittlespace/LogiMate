//go:build windows

package main

import (
	"github.com/thelittlespace/LogiMate/internal/app"
	"os"
	"strings"
)

var version = "0.0.1-alpha"
var buildID = "dev"

func main() {
	args := os.Args[1:]
	if len(args) >= 1 && args[0] == "--uninstall" {
		os.Exit(app.Uninstall())
	}
	if len(args) >= 1 && args[0] == "--uninstall-admin" {
		os.Exit(app.UninstallAdmin())
	}
	if len(args) >= 2 && args[0] == "--admin-action" {
		action := args[1]
		data := ""
		migrationToken := ""
		for i := 2; i < len(args)-1; i++ {
			switch args[i] {
			case "--data":
				data = strings.Trim(args[i+1], "\"")
			case "--migration-token":
				migrationToken = strings.Trim(args[i+1], "\"")
			}
		}
		os.Exit(app.AdminAction(action, data, migrationToken))
	}
	app.Version = version
	app.BuildID = buildID
	os.Exit(app.Run())
}
