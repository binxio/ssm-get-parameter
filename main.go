//
//   Licensed under the Apache License, Version 2.0 (the "License");
//   you may not use this file except in compliance with the License.
//   You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
//   Unless required by applicable law or agreed to in writing, software
//   distributed under the License is distributed on an "AS IS" BASIS,
//   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//   See the License for the specific language governing permissions and
//   limitations under the License.
//
//   Copyright 2018 Binx.io B.V.
package main

import (
	"bytes"
	"flag"
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ssm"
	"io/ioutil"
	"log"
	"net/url"
	"os"
	"os/exec"
	"strings"
	"syscall"
	template "text/template"
)

func main() {
	var name string
	flag.StringVar(&name, "parameter-name", "", "of the parameter (deprecated)")
	flag.StringVar(&name, "name", "", "of the parameter")
	flag.Parse()
	if name != "" {
		getParameter(name)
	} else {
		if len(os.Args) <= 1 {
			log.Fatalf("ERROR: expected --name or a command to run")
		}
		execProcess(os.Args[1:])
	}
}

type SSMParameterRef struct {
	name           *string            // of the environment variable
	parameter_name *string            // in the parameter store
	default_value  *string            // if one is specified
	destination    *string            // to write the value to, otherwise ""
	template       *template.Template // to use, defaults to '{{.}}'
}

// converts the environment variables in `environ` into a list of SSM parameter references.
func environmentToSSMParameterReferences(environ []string) ([]SSMParameterRef, error) {
	result := make([]SSMParameterRef, 0, 10)
	for i := 0; i < len(environ); i++ {
		name, value := toNameValue(environ[i])
		if strings.HasPrefix(value, "ssm:") {
			value := os.ExpandEnv(value)
			uri, err := url.Parse(value)
			if err != nil {
				return nil, fmt.Errorf("failed to parse environment variable %s, %s", name, err)
			}
			if uri.Host != "" {
				return nil, fmt.Errorf("environment variable %s has an ssm: uri, but specified a host. add a /.", name)
			}
			values, err := url.ParseQuery(uri.RawQuery)
			if err != nil {
				return nil, fmt.Errorf("environment variable %s has an invalid query syntax, %s", name, err)
			}

			defaultValue := values.Get("default")
			destination := values.Get("destination")
			var tpl *template.Template
			if values.Get("template") != "" {
				tpl, err = template.New("secret").Parse(values.Get("template"))
				if err != nil {
					return nil, fmt.Errorf("environment variable %s has an invalid template syntax, %s", name, err)
				}
			}
			result = append(result, SSMParameterRef{&name, &uri.Path,
				&defaultValue, &destination, tpl})
		}
	}
	return result, nil
}

// get the default value for the parameter
func getDefaultValue(ref *SSMParameterRef) (string, error) {
	if *ref.default_value != "" {
		if ref.template != nil {
			return formatValue(ref, ref.default_value), nil
		}
		return *ref.default_value, nil
	}

	if *ref.destination != "" {
		content, err := ioutil.ReadFile(*ref.destination)
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
		result[*ref.name] = *ref.default_value
		request := ssm.GetParameterInput{Name: ref.parameter_name, WithDecryption: &withDecryption}
		response, err := service.GetParameter(&request)
		if err == nil {
			result[*ref.name] = formatValue(&ref, response.Parameter.Value)
		} else {
			msg := fmt.Sprintf("failed to get parameter %s, %s", *ref.name, err)
			ssmError, ok := err.(awserr.Error)
			if ok && ssmError.Code() == ssm.ErrCodeParameterNotFound {
				value, err := getDefaultValue(&ref)
				if err != nil {
					return nil, fmt.Errorf("ERROR: %s, %s\n", msg, err)
				}
				result[*ref.name] = value
			} else {
				return nil, fmt.Errorf("ERROR: %s\n", msg)
			}
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
	newEnv, err := ssmParameterReferencesToEnvironment(refs)
	if err != nil {
		log.Fatal(err)
	}

	err = writeParameterValues(refs, newEnv)
	if err != nil {
		log.Fatal(err)
	}

	newEnv = replaceDestinationReferencesWithURL(refs, newEnv)

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
	session, err := session.NewSession(aws.NewConfig())
	if err != nil {
		log.Fatalf("ERROR: failed to create new session %s\n", err)
	}
	return session
}

// get the name and variable of a environment entry in the form of <name>=<value>
func toNameValue(envEntry string) (string, string) {
	result := strings.SplitN(envEntry, "=", 2)
	return result[0], result[1]
}
