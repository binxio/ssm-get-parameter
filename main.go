// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//
// Copyright 2018-2023 Binx.io B.V.
package main

import (
	"bytes"
	"flag"
	"fmt"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ssm"
	"github.com/mitchellh/go-homedir"
	"log"
	"net/url"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"text/template"
)

var verbose bool

func main() {
	var name string
	var export bool
	flag.StringVar(&name, "parameter-name", "", "of the parameter (deprecated)")
	flag.StringVar(&name, "name", "", "of the parameter")
	flag.BoolVar(&export, "export", false, "all environment parameter references")
	flag.BoolVar(&verbose, "verbose", false, "get debug output")
	flag.Parse()
	if name != "" {
		getParameter(name)
	} else if export {
		exportSSMReferences()
	} else {
		if len(os.Args) <= 1 {
			log.Fatalf("ERROR: expected --name, --export or a command to run")
		}
		execProcess(flag.Args())
	}
}

type SSMParameterRef struct {
	name          *string            // of the environment variable
	parameterName *string            // in the parameter store
	defaultValue  *string            // if one is specified
	destination   *string            // to write the value to, otherwise ""
	fileMode      os.FileMode        // file permissions
	template      *template.Template // to use, defaults to '{{.}}'
}

// CreateSSMParameterRef creates a SSM parameter reference from a environment value.
func CreateSSMParameterRef(name, value string) (result *SSMParameterRef, err error) {
	value = os.ExpandEnv(value)
	uri, err := url.Parse(value)
	if err != nil {
		return nil, fmt.Errorf("failed to parse environment variable %s, %s", name, err)
	}
	if uri.Host != "" {
		return nil, fmt.Errorf("environment variable %s has an ssm: uri, but specified a host. add a /", name)
	}
	values, err := url.ParseQuery(uri.RawQuery)
	if err != nil {
		return nil, fmt.Errorf("environment variable %s has an invalid query syntax, %s", name, err)
	}

	defaultValue := values.Get("default")
	destination, err := homedir.Expand(values.Get("destination"))
	if err != nil {
		return nil, err
	}
	var tpl *template.Template
	if values.Get("template") != "" {
		tpl, err = template.New("secret").Parse(values.Get("template"))
		if err != nil {
			return nil, fmt.Errorf("environment variable %s has an invalid template syntax, %s", name, err)
		}
	}
	fileMode := os.FileMode(0)
	chmod := values.Get("chmod")
	if chmod != "" {
		if mode, err := strconv.ParseUint(chmod, 8, 32); err != nil {
			return nil, fmt.Errorf("chmod '%s' is not valid, %s", chmod, err)
		} else {
			fileMode = os.FileMode(mode)
		}
	}
	result = &SSMParameterRef{
		&name,
		&uri.Path,
		&defaultValue,
		&destination,
		os.FileMode(fileMode),
		tpl}

	return result, nil
}

// converts the environment variables in `environ` into a list of SSM parameter references.
func environmentToSSMParameterReferences(environ []string) ([]SSMParameterRef, error) {
	result := make([]SSMParameterRef, 0, 10)
	for i := 0; i < len(environ); i++ {
		name, value := toNameValue(environ[i])
		if strings.HasPrefix(value, "ssm:") {
			ref, err := CreateSSMParameterRef(name, value)
			if err != nil {
				return nil, err
			}
			result = append(result, *ref)
		}
	}
	return result, nil
}

// get the default value for the parameter
func getDefaultValue(ref *SSMParameterRef) (string, error) {
	if *ref.defaultValue != "" {
		if ref.template != nil {
			return formatValue(ref, ref.defaultValue), nil
		}
		return *ref.defaultValue, nil
	}

	if *ref.destination != "" {
		content, err := os.ReadFile(*ref.destination)
		if err == nil {
			return string(content), nil
		}
		return "", fmt.Errorf("destination file does not exist to provide default value")
	}
	return "", fmt.Errorf("no default value available")
}

// retrieve all the parameter store values from refs and return the result as a name-value map.
func ssmParameterReferencesToEnvironment(refs []SSMParameterRef) (map[string]string, error) {
	result := make(map[string]string)
	withDecryption := true
	service := ssm.New(getSession())
	for _, ref := range refs {
		result[*ref.name] = *ref.defaultValue
		request := ssm.GetParameterInput{Name: ref.parameterName, WithDecryption: &withDecryption}
		response, err := service.GetParameter(&request)
		if err == nil {
			result[*ref.name] = formatValue(&ref, response.Parameter.Value)
		} else {
			msg := fmt.Sprintf("failed to get parameter %s, %s", *ref.name, err)
			if verbose {
				log.Printf("WARNING: %s", msg)
			}
			value, err := getDefaultValue(&ref)
			if err != nil {
				return nil, fmt.Errorf("ERROR: %s, %s\n", msg, err)
			}
			result[*ref.name] = value
		}

	}
	return result, nil
}

func formatValue(ref *SSMParameterRef, value *string) string {
	var writer bytes.Buffer
	if ref.template == nil {
		return *value
	}

	if err := ref.template.Execute(&writer, value); err != nil {
		log.Fatalf("failed to format value of '%s' with template", *ref.name)
	}
	return writer.String()
}

// create a new environment from `env` with new values from `newEnv`
func updateEnvironment(env []string, newEnv map[string]string) []string {
	result := make([]string, 0, len(env))
	for i := 0; i < len(env); i++ {
		name, _ := toNameValue(env[i])
		if newValue, ok := newEnv[name]; ok {
			result = append(result, fmt.Sprintf("%s=%s", name, newValue))
		} else {
			result = append(result, env[i])
		}
	}
	return result
}

// write the value of each reference to the specified destination file
func writeParameterValues(refs []SSMParameterRef, env map[string]string) error {
	for _, ref := range refs {
		if *ref.destination != "" {

			f, err := os.Create(*ref.destination)
			if err != nil {
				return fmt.Errorf("failed to open file %s to write to, %s", *ref.destination, err)
			}
			_, err = f.WriteString(env[*ref.name])
			if err != nil {
				return fmt.Errorf("failed to write to file %s, %s", *ref.destination, err)
			}
			err = f.Close()
			if err != nil {
				return fmt.Errorf("failed to close file %s, %s", *ref.destination, err)
			}

			if ref.fileMode != 0 {
				err := os.Chmod(*ref.destination, ref.fileMode)
				if err != nil {
					return fmt.Errorf("failed to chmod file %s to %s, %s", *ref.destination, ref.fileMode, err)
				}
			}
		}
	}
	return nil
}
func replaceDestinationReferencesWithURL(refs []SSMParameterRef, env map[string]string) map[string]string {
	for _, ref := range refs {
		if *ref.destination != "" {
			env[*ref.name] = fmt.Sprintf("%s", *ref.destination)
		}
	}
	return env
}

// resolves SSM parameter references to values
func resolveSSMParameterReferences(refs []SSMParameterRef) (map[string]string, error) {
	newEnv, err := ssmParameterReferencesToEnvironment(refs)
	if err != nil {
		return nil, err
	}

	err = writeParameterValues(refs, newEnv)
	if err != nil {
		return nil, err
	}

	return replaceDestinationReferencesWithURL(refs, newEnv), nil
}

// execute the `cmd` with the environment set to actual values from the parameter store
func execProcess(cmd []string) {
	program, err := exec.LookPath(cmd[0])
	if err != nil {
		log.Fatalf("could not find program %s on path, %s", cmd[0], err)
	}

	refs, err := environmentToSSMParameterReferences(os.Environ())
	if err != nil {
		log.Fatal(err)
	}
	newEnv, err := resolveSSMParameterReferences(refs)
	if err != nil {
		log.Fatal(err)
	}

	err = syscall.Exec(program, cmd, updateEnvironment(os.Environ(), newEnv))
	if err != nil {
		log.Fatal(err)
	}
}

// write the value of the parameter `name` to stdout.
func getParameter(name string) {
	withDecryption := true
	service := ssm.New(getSession())
	request := ssm.GetParameterInput{Name: &name, WithDecryption: &withDecryption}
	response, err := service.GetParameter(&request)
	if err != nil {
		log.Fatalf("ERROR: failed to get parameter, %s\n", err)
	}
	_, err = fmt.Printf("%s", *response.Parameter.Value)
	if err != nil {
		log.Fatalf("ERROR: failed to write value, %s\n", err)
	}
}

// get a new AWS Session
func getSession() *session.Session {
	s, err := session.NewSessionWithOptions(session.Options{
		SharedConfigState: session.SharedConfigEnable, // Must be set to enable
	})
	if err != nil {
		log.Fatalf("ERROR: failed to create new session %s\n", err)
	}
	return s
}

// get the name and variable of an environment entry in the form of <name>=<value>
func toNameValue(envEntry string) (string, string) {
	result := strings.SplitN(envEntry, "=", 2)
	return result[0], result[1]
}

func exportSSMReferences() {
	refs, err := environmentToSSMParameterReferences(os.Environ())
	if err != nil {
		log.Fatal(err)
	}
	newEnv, err := resolveSSMParameterReferences(refs)
	if err != nil {
		log.Fatal(err)
	}
	for _, ref := range refs {
		value := strings.ReplaceAll(newEnv[*ref.name], "'", "'\"'\"'")
		fmt.Printf("%s='%s'; export %s\n", *ref.name, value, *ref.name)
	}
}
