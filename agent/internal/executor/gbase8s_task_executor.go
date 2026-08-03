package executor

import (
	"context"
	"fmt"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

const gbase8sRoot = "/opt/dbops/gbase8s"

var gbase8sPassword = regexp.MustCompile(`^[A-Za-z0-9!@#%+=._-]{12,128}$`)
var gbase8sParameters = map[string]*regexp.Regexp{
	"BUFFERS":      regexp.MustCompile(`^[1-9][0-9]{0,6}$`),
	"DYNAMIC_LOGS": regexp.MustCompile(`^[01]$`),
	"LOCKS":        regexp.MustCompile(`^[1-9][0-9]{0,6}$`),
	"LOGFILES":     regexp.MustCompile(`^[1-9][0-9]{0,3}$`),
}

func (e *FlavorTaskExecutor) executeGBase8s(ctx context.Context, req FlavorTaskRequest, manifest LocalPackageManifest) (*TaskResult, error) {
	if err := validateGBase8sConfig(req); err != nil {
		return flavorTaskFailure(req.TaskID, err), nil
	}
	if req.Operation != "deploy" && req.Operation != "configure" {
		return flavorTaskFailure(req.TaskID, fmt.Errorf("gbase8s single-node executor only supports deploy and configure; %s is unavailable", req.Operation)), nil
	}
	if req.Operation == "deploy" {
		if !gbase8sInstallerAvailable(manifest) {
			return flavorTaskFailure(req.TaskID, fmt.Errorf("gbase8s bundle requires manifest-verified PluginPak/install_init.sh")), nil
		}
		if _, err := e.handlers["gbase8s"].runner(ctx, "sh", "-c", "cd \"$1/PluginPak\" && sh ./install_init.sh", "sh", req.PackagePath); err != nil {
			return flavorTaskFailure(req.TaskID, fmt.Errorf("install gbase8s with PluginPak/install_init.sh: %w", err)), nil
		}
		if _, err := e.handlers["gbase8s"].runner(ctx, gbase8sBinary(req, "oninit"), "-vy"); err != nil {
			return flavorTaskFailure(req.TaskID, fmt.Errorf("initialize gbase8s instance: %w", err)), nil
		}
	}
	if err := e.configureGBase8s(ctx, req); err != nil {
		return flavorTaskFailure(req.TaskID, err), nil
	}
	if req.Operation == "deploy" {
		if err := e.createGBase8sApplicationUser(ctx, req); err != nil {
			return flavorTaskFailure(req.TaskID, err), nil
		}
	}
	if err := e.verifyGBase8s(ctx, req); err != nil {
		return flavorTaskFailure(req.TaskID, err), nil
	}
	return flavorTaskCompleted(req.TaskID, "gbase8s single-node "+req.Operation+" completed", map[string]interface{}{"flavor": "gbase8s", "version": req.Version}), nil
}

func validateGBase8sConfig(req FlavorTaskRequest) error {
	c := req.GBase8s
	if c == nil || !gbase8sPath(c.InstallDir) || !tidbIdentifier.MatchString(c.ApplicationUser) || !gbase8sPassword.MatchString(c.ApplicationPassword) {
		return fmt.Errorf("gbase8s deployment inputs are invalid")
	}
	if req.TLS.Enabled {
		return fmt.Errorf("gbase8s TLS is unavailable because no public, verifiable TLS command is configured")
	}
	for _, parameters := range []map[string]string{c.PersistentParameters, c.MemoryParameters} {
		for key, value := range parameters {
			validator, allowed := gbase8sParameters[key]
			if !allowed || !validator.MatchString(value) {
				return fmt.Errorf("gbase8s parameter %q is not allowed", key)
			}
		}
	}
	return nil
}

func gbase8sPath(path string) bool {
	clean := filepath.Clean(path)
	return filepath.IsAbs(path) && strings.HasPrefix(clean, gbase8sRoot+string(filepath.Separator)) && !strings.Contains(path, "..")
}

func gbase8sInstallerAvailable(manifest LocalPackageManifest) bool {
	for _, artifact := range manifest.Packages {
		if artifact.File == "PluginPak/install_init.sh" {
			return true
		}
	}
	return false
}

func gbase8sBinary(req FlavorTaskRequest, binary string) string {
	return filepath.Join(req.GBase8s.InstallDir, "bin", binary)
}

func (e *FlavorTaskExecutor) configureGBase8s(ctx context.Context, req FlavorTaskRequest) error {
	c := req.GBase8s
	for _, update := range []struct {
		parameters map[string]string
		mode       string
	}{
		{c.PersistentParameters, "-wf"},
		{c.MemoryParameters, "-wm"},
	} {
		keys := make([]string, 0, len(update.parameters))
		for key := range update.parameters {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			if _, err := e.handlers["gbase8s"].runner(ctx, gbase8sBinary(req, "onmode"), update.mode, key+"="+update.parameters[key]); err != nil {
				return fmt.Errorf("set gbase8s parameter %s: %w", key, err)
			}
		}
	}
	return nil
}

func (e *FlavorTaskExecutor) createGBase8sApplicationUser(ctx context.Context, req FlavorTaskRequest) error {
	c := req.GBase8s
	statement := "CREATE USER " + c.ApplicationUser + " WITH PASSWORD '" + c.ApplicationPassword + "';"
	if _, err := e.inputRunner(ctx, statement+"\n", gbase8sBinary(req, "dbaccess"), "-", "-"); err != nil {
		return fmt.Errorf("create gbase8s application user: %w", err)
	}
	return nil
}

func (e *FlavorTaskExecutor) verifyGBase8s(ctx context.Context, req FlavorTaskRequest) error {
	for _, args := range [][]string{{"-c"}, {"-g", "cfg"}} {
		if _, err := e.handlers["gbase8s"].runner(ctx, gbase8sBinary(req, "onstat"), args...); err != nil {
			return fmt.Errorf("verify gbase8s onstat %s: %w", strings.Join(args, " "), err)
		}
	}
	if _, err := e.inputRunner(ctx, "SELECT DBINFO('version', 'full') FROM systables WHERE tabid = 1;\n", gbase8sBinary(req, "dbaccess"), "-", "-"); err != nil {
		return fmt.Errorf("verify gbase8s DB-Access query: %w", err)
	}
	return nil
}
