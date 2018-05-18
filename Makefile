include Makefile.mk

BINARIES=dist/ssm-get-parameter-$(VERSION)-linux-amd64.zip dist/ssm-get-parameter-$(VERSION)-darwin-amd64.zip
REPO_URL=https://api.github.com/repos/binxio/ssm-get-parameter

build: $(BINARIES)

dist/ssm-get-parameter-$(VERSION)-linux-amd64.zip: main.go
	mkdir -p dist
	GOOS=linux GOARCH=amd64 go build -o dist/ssm-get-parameter main.go
	cd dist && zip ../dist/ssm-get-parameter-$(VERSION)-linux-amd64.zip ssm-get-parameter  && rm ssm-get-parameter

dist/ssm-get-parameter-$(VERSION)-darwin-amd64.zip: main.go
	mkdir -p dist
	GOOS=darwin GOARCH=amd64 go build -o dist/ssm-get-parameter main.go
	cd dist && zip ../dist/ssm-get-parameter-$(VERSION)-darwin-amd64.zip ssm-get-parameter  && rm ssm-get-parameter

clean:
	rm -rf dist target



.git-release-$(VERSION):
	set -e -o pipefail ; \
	jq \
		-n \
		--arg tag $(TAG) \
		--arg release $(VERSION) \
		'{ "target_commitish": "master", "draft": true, "prerelease": false,  "tag_name": $$tag, "name": $$release, "body": ("release " + $$release) }' | \
	tee /dev/tty | \
	curl -s \
			-d @- \
			-o .git-release-$(VERSION) \
			-H "Authorization: token $$GITHUB_API_TOKEN" \
			-H 'Content-Type: application/json' \
			$(REPO_URL)/releases

release: .git-release-$(VERSION) $(BINARIES)
	for BINARY in $(BINARIES); do \
		curl -s \
			 --data-binary @$$BINARY \
			-o /dev/null \
			-X POST \
			-H "Authorization: token $$GITHUB_API_TOKEN" \
			-H 'Content-Type: application/octet-stream' \
			$(REPO_URL)/releases/$(shell jq -r .id .git-release-$(VERSION))/assets?name=$$(basename $${BINARY} | sed -e 's/-$(VERSION)-/-/') ; \
	done
	curl --fail --silent \
		-d '{"draft": false}'  \
		-X PATCH \
		-o /dev/null \
		-H "Authorization: token $$GITHUB_API_TOKEN" \
		-H 'Content-Type: application/json-stream' \
		$(REPO_URL)/releases/$(shell jq -r .id .git-release-$(VERSION))
	rm .git-release-$(VERSION)
