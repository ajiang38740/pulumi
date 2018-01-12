// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package integration

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/pulumi/pulumi/pkg/util/cmdutil"
	pioutil "github.com/pulumi/pulumi/pkg/util/ioutil"
)

// RunCommand executes the specified command and additional arguments, wrapping any output in the
// specialized test output streams that list the location the test is running in.
func RunCommand(t *testing.T, name string, args []string, wd string, opts *ProgramTestOptions) error {
	path := args[0]
	command := strings.Join(args, " ")
	pioutil.MustFprintf(opts.Stdout, "**** Invoke '%v' in '%v'\n", command, wd)

	// Spawn a goroutine to print out "still running..." messages.
	finished := false
	go func() {
		for !finished {
			time.Sleep(30 * time.Second)
			if !finished {
				pioutil.MustFprintf(opts.Stderr, "Still running command '%s' (%s)...\n", command, wd)
			}
		}
	}()

	var env []string
	env = append(env, os.Environ()...)
	env = append(env, "PULUMI_RETAIN_CHECKPOINTS=true")
	env = append(env, "PULUMI_CONFIG_PASSPHRASE=correct horse battery staple")

	cmd := exec.Cmd{
		Path: path,
		Dir:  wd,
		Args: args,
		Env:  env,
	}

	startTime := time.Now()

	var runout []byte
	var runerr error
	if opts.Verbose || os.Getenv("PULUMI_VERBOSE_TEST") != "" {
		cmd.Stdout = opts.Stdout
		cmd.Stderr = opts.Stderr
		runerr = cmd.Run()
	} else {
		runout, runerr = cmd.CombinedOutput()
	}

	endTime := time.Now()

	if opts.ReportStats != nil {
		// Note: This data is archived and used by external analytics tools.  Take care if changing the schema or format
		// of this data.
		opts.ReportStats.ReportCommand(TestCommandStats{
			StartTime:      startTime.Format("2006/01/02 15:04:05"),
			EndTime:        endTime.Format("2006/01/02 15:04:05"),
			ElapsedSeconds: float64((endTime.Sub(startTime)).Nanoseconds()) / 1000000000,
			StepName:       name,
			CommandLine:    command,
			StackName:      string(opts.GetStackName()),
			TestID:         wd,
			TestName:       filepath.Base(opts.Dir),
			IsError:        runerr != nil,
		})
	}

	finished = true
	if runerr != nil {
		pioutil.MustFprintf(opts.Stderr, "Invoke '%v' failed: %s\n", command, cmdutil.DetailedError(runerr))

		if !opts.Verbose {
			// We've seen long fprintf's fail on Travis, so avoid panicing.
			if _, err := fmt.Fprintf(opts.Stderr, "%s\n", string(runout)); err != nil {
				pioutil.MustFprintf(opts.Stderr, "\n\nOutput truncated: %v\n", err)
			}
		}
	}

	// If we collected any program output, write it to a log file -- success or failure.
	if len(runout) > 0 {
		if logFile, err := writeCommandOutput(name, wd, runout); err != nil {
			pioutil.MustFprintf(opts.Stderr, "Failed to write output: %v\n", err)
		} else {
			pioutil.MustFprintf(opts.Stderr, "Wrote output to %s\n", logFile)
		}
	}

	return runerr
}

func withOptionalYarnFlags(args []string) []string {
	flags := os.Getenv("YARNFLAGS")

	if flags != "" {
		return append(args, flags)
	}

	return args
}

// addFlagIfNonNil will take a set of command-line flags, and add a new one if the provided flag value is not empty.
func addFlagIfNonNil(args []string, flag, flagValue string) []string {
	if flagValue != "" {
		args = append(args, flag, flagValue)
	}
	return args
}