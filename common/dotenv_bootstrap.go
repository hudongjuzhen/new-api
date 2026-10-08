package common

import (
	"os"
	"path/filepath"

	"github.com/joho/godotenv"
)

// Load the host's .env before any package resolves configuration at package level.
//
// # Why this lives in `common` and not only in main()
//
// Go initializes *imported* packages before the importing package's own
// package-level variables, and all of that before main() runs. The host loads
// .env inside main() -> InitResources(), which is therefore too late for any
// plugin that resolves its config in a package-level var:
//
//	// zsy/world/config.go
//	var cfg = Config{
//	    EnginePath: common.GetEnvOrDefaultString(envEnginePath, EnginePathDefault),
//	    …
//	}
//
// The symptom is nasty to diagnose — the value IS in .env, it looks configured,
// and the plugin silently keeps its compiled-in placeholder ("a placeholder
// default, not a working one"), so the opaque failure an operator sees is:
//
//	世界引擎不可用：找不到或无法执行 world-parser-svc（GetFileAttributesEx …）
//
// Every plugin imports `common`, so an init() here is guaranteed (dependency
// order) to run before any of them resolves its own package-level config. That
// also fixes it for future plugins instead of patching one file at a time.
//
// godotenv.Load never overwrites a variable that is already set, so:
//   - calling it again from main() stays harmless;
//   - a real process environment (docker, systemd, a shell export) still wins
//     over the file — the precedence operators expect.
//
// Order mirrors main(): prefer .env in the working directory, and fall back to
// the directory of the executable, so the server starts correctly no matter
// which directory it is launched from.
func init() {
	if err := godotenv.Load(".env"); err != nil {
		if exe, exeErr := os.Executable(); exeErr == nil {
			_ = godotenv.Load(filepath.Join(filepath.Dir(exe), ".env"))
		}
	}
}
