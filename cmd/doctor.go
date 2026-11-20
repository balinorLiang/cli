package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/afero"
	"github.com/spf13/cobra"
	"github.com/supabase/cli/internal/highgobase"
	"github.com/supabase/cli/internal/utils"
	"github.com/supabase/cli/internal/utils/flags"
	"github.com/supabase/cli/pkg/config"
)

type doctorCheck struct {
	Name   string `json:"name"`
	OK     bool   `json:"ok"`
	Fatal  bool   `json:"-"`
	Detail string `json:"detail,omitempty"`
}

var (
	doctorJSON       bool
	doctorSkipDocker bool
	doctorCmd        = &cobra.Command{
		Use:   "doctor",
		Short: "Check HighgoBase CLI prerequisites",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := flags.LoadConfig(afero.NewOsFs()); err != nil {
				return err
			}
			checks := highgobaseChecks(cmd.Context())
			return printDoctorChecks(os.Stdout, checks)
		},
	}
)

func init() {
	if !highgobase.Enabled() {
		return
	}
	flagSet := doctorCmd.Flags()
	flagSet.BoolVar(&doctorJSON, "json", false, "Output checks as JSON")
	flagSet.BoolVar(&doctorSkipDocker, "skip-docker", false, "Skip Docker daemon and image checks")
	rootCmd.AddCommand(doctorCmd)
}

func highgobaseChecks(ctx context.Context) []doctorCheck {
	var checks []doctorCheck
	checks = append(checks, doctorCheck{
		Name:   "higobase home",
		OK:     true,
		Fatal:  true,
		Detail: highgobase.Home(),
	})
	for _, path := range highgobase.RequiredFiles() {
		check := doctorCheck{Name: "asset " + path, Fatal: true}
		if _, err := os.Stat(path); err == nil {
			check.OK = true
		} else {
			check.Detail = err.Error()
		}
		checks = append(checks, check)
	}
	license, ok := highgobase.LicenseDir()
	licenseCheck := doctorCheck{Name: "hgdb license", OK: ok, Fatal: false}
	if ok {
		licenseCheck.Detail = license
	} else {
		licenseCheck.Detail = "not found in " + license + " (required by the hgdb image)"
	}
	checks = append(checks, licenseCheck)
	checks = append(checks, doctorCheck{
		Name:   "database image",
		OK:     true,
		Fatal:  true,
		Detail: highgobase.ImageDB(),
	})
	checks = append(checks, doctorCheck{
		Name:   "database version",
		OK:     utils.Config.Db.MajorVersion == 14,
		Fatal:  false,
		Detail: fmt.Sprintf("major_version=%d (HighgoBase 9.x is PostgreSQL 14 compatible)", utils.Config.Db.MajorVersion),
	})
	if doctorSkipDocker {
		return checks
	}
	if _, err := utils.Docker.Ping(ctx); err != nil {
		checks = append(checks, doctorCheck{Name: "docker daemon", Fatal: true, Detail: err.Error()})
		return checks
	}
	checks = append(checks, doctorCheck{Name: "docker daemon", OK: true, Fatal: true})
	for _, image := range highgobase.LocalImages() {
		required := highgobase.ImageRequired(image, config.Config(&utils.Config))
		check := doctorCheck{Name: "image " + image, Fatal: required}
		if _, err := utils.Docker.ImageInspect(ctx, utils.GetRegistryImageUrl(image)); err == nil {
			check.OK = true
		} else if !required {
			check.Detail = "not loaded locally (service disabled)"
		} else {
			check.Detail = "not loaded locally"
		}
		checks = append(checks, check)
	}
	return checks
}

func printDoctorChecks(w *os.File, checks []doctorCheck) error {
	failed := false
	for _, check := range checks {
		if check.Fatal && !check.OK {
			failed = true
		}
	}
	if doctorJSON {
		payload := struct {
			OK     bool          `json:"ok"`
			Checks []doctorCheck `json:"checks"`
		}{OK: !failed, Checks: checks}
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		if err := enc.Encode(payload); err != nil {
			return err
		}
	} else {
		for _, check := range checks {
			status := "WARN"
			if check.OK {
				status = "OK"
			} else if check.Fatal {
				status = "FAIL"
			}
			fmt.Fprintf(w, "[%s] %s\n", status, check.Name)
			if len(check.Detail) > 0 {
				fmt.Fprintf(w, "       %s\n", check.Detail)
			}
		}
	}
	if failed {
		return fmt.Errorf("HighgoBase CLI prerequisites are not satisfied")
	}
	return nil
}
