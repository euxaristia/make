package main

import (
	"reflect"
	"testing"
)

func TestParseArgs(t *testing.T) {
	tests := []struct {
		name    string
		argv    []string
		want    *CliArgs
		wantErr string
	}{
		{
			name: "defaults",
			want: &CliArgs{
				Jobs:      1,
				Overrides: map[string]string{},
			},
		},
		{
			name: "flags target and overrides",
			argv: []string{"-f", "custom.mk", "-j", "4", "-k", "-i", "-s", "-q", "-p", "-n", "-t", "-e", "build", "CC=clang"},
			want: &CliArgs{
				Target:      "build",
				Makefile:    "custom.mk",
				Jobs:        4,
				KeepGoing:   true,
				IgnoreErrs:  true,
				Silent:      true,
				Question:    true,
				PrintDB:     true,
				DryRun:      true,
				Touch:       true,
				EnvOverride: true,
				Overrides: map[string]string{
					"CC": "clang",
				},
			},
		},
		{
			name: "S cancels keep going",
			argv: []string{"-k", "-S"},
			want: &CliArgs{
				Jobs:      1,
				Overrides: map[string]string{},
			},
		},
		{
			name:    "missing makefile argument",
			argv:    []string{"-f"},
			wantErr: "-f requires an argument",
		},
		{
			name:    "invalid jobs",
			argv:    []string{"-j", "0"},
			wantErr: "invalid -j value",
		},
		{
			name:    "unknown option",
			argv:    []string{"--wat"},
			wantErr: "unknown option: --wat",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, gotErr := parseArgs(tt.argv)
			if gotErr != tt.wantErr {
				t.Fatalf("parseArgs() error = %q, want %q", gotErr, tt.wantErr)
			}
			if tt.wantErr != "" {
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("parseArgs() = %#v, want %#v", got, tt.want)
			}
		})
	}
}
