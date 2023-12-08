package main

import (
	"fmt"
	"github.com/davecgh/go-spew/spew"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func pointer[T any](v T) *T { return &v }

func TestCreateSSMParameterRef(t *testing.T) {

	tempDir := t.TempDir()

	type args struct {
		name  string
		value string
	}
	tests := []struct {
		name       string
		args       args
		wantResult *SSMParameterRef
		wantErr    error
	}{{
		name: "simple",
		args: args{"PRIVATE_KEY_FILE", "ssm:///private-key?destination=./.id_rsa"},
		wantResult: &SSMParameterRef{
			pointer("PRIVATE_KEY_FILE"),
			pointer("/private-key"),
			pointer(""),
			pointer("./.id_rsa"),
			os.FileMode(int(0)),
			nil,
		},
		wantErr: nil,
	}, {
		name: "home",
		args: args{"PRIVATE_KEY_FILE", "ssm:///private-key?destination=~/.id_rsa"},
		wantResult: &SSMParameterRef{
			pointer("PRIVATE_KEY_FILE"),
			pointer("/private-key"),
			pointer(""),
			pointer(fmt.Sprintf("%s/.id_rsa", tempDir)),
			os.FileMode(int(0)),
			nil,
		},
		wantErr: nil,
	}, {
		name: "with chmod",
		args: args{"PRIVATE_KEY_FILE", "ssm:///private-key?destination=~/.id_rsa&chmod=600"},
		wantResult: &SSMParameterRef{
			pointer("PRIVATE_KEY_FILE"),
			pointer("/private-key"),
			pointer(""),
			pointer(fmt.Sprintf("%s/.id_rsa", tempDir)),
			os.FileMode(int(0600)),
			nil,
		},
		wantErr: nil,
	},
		{
			name: "invalid home",
			args: args{"PRIVATE_KEY_FILE", "ssm:///private-key?destination=~root/.id_rsa&chmod=755"},
			wantResult: &SSMParameterRef{
				pointer("PRIVATE_KEY_FILE"),
				pointer("/private-key"),
				pointer(""),
				pointer(fmt.Sprintf("%s/.id_rsa", tempDir)),
				os.FileMode(int(0755)),
				nil,
			},
			wantErr: fmt.Errorf("cannot expand user-specific home dir"),
		},
	}
	t.Setenv("HOME", tempDir)

	filename := filepath.Join(tempDir, ".id_rsa")
	err := os.WriteFile(filename, []byte("rsa key"), os.FileMode(int(0600)))
	if err != nil {
		t.Fatal(err)
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotResult, err := CreateSSMParameterRef(tt.args.name, tt.args.value)
			if (err != nil) != (tt.wantErr != nil) ||
				(err != nil && tt.wantErr != nil && err.Error() != tt.wantErr.Error()) {
				t.Errorf("CreateSSMParameterRef() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if err == nil && !reflect.DeepEqual(tt.wantResult, gotResult) {

				msg, _ := spew.Printf("CreateSSMParameterRef()\ngot : %v\nwant: %v\n", gotResult, tt.wantResult)
				t.Error(msg)
			}
		})
	}
}
