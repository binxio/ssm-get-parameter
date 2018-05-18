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
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ssm"
	"flag"
	"github.com/aws/aws-sdk-go/aws"
	"fmt"
	"os"
)

func main() {
	withDecryption := true
	name := flag.String("parameter-name", "", "the name of the parameter")
	flag.Parse()

	if *name == "" {
		fmt.Fprintf(os.Stderr,"ERROR: missing option --parameter-name\n")
		os.Exit(1)
	}
	session, err := session.NewSession(aws.NewConfig())
	if err != nil {
		fmt.Fprintf(os.Stderr,"ERROR: failed to create new session %s\n", err)
		os.Exit(1)
	}

	service := ssm.New(session)
	request := ssm.GetParameterInput{Name: name, WithDecryption: &withDecryption}
	response, err := service.GetParameter(&request)
	if err != nil {
		fmt.Fprintf(os.Stderr,"ERROR: failed to get parameter, %s\n", err)
		os.Exit(1)
	}

	fmt.Printf("%s", *response.Parameter.Value)
}
