# The short Git commit hash
SHORT_COMMIT := $(shell git rev-parse --short HEAD)
BRANCH_NAME:=$(shell git rev-parse --abbrev-ref HEAD | tr '/' '-')
# The Git commit hash
COMMIT := $(shell git rev-parse HEAD)
# The tag of the current commit, otherwise empty
VERSION := $(shell git describe --tags --abbrev=2 --match "v*" --match "secure-cadence*" 2>/dev/null)

# By default, this will run all tests in all packages, but we have a way to override this in CI so that we can
# dynamically split up CI jobs into smaller jobs that can be run in parallel
GO_TEST_PACKAGES := ./...

# Image tag: if image tag is not set, set it with version (or short commit if empty)
ifeq (${IMAGE_TAG},)
IMAGE_TAG := ${VERSION}
endif

ifeq (${IMAGE_TAG},)
IMAGE_TAG := ${SHORT_COMMIT}
endif

IMAGE_TAG_NO_ADX := $(IMAGE_TAG)-without-adx
IMAGE_TAG_NO_NETGO_NO_ADX := $(IMAGE_TAG)-without-netgo-without-adx
IMAGE_TAG_ARM := $(IMAGE_TAG)-arm

# Name of the cover profile
COVER_PROFILE := coverage.txt
# Disable go sum database lookup for private repos
GOPRIVATE=github.com/dapperlabs/*
# OS
UNAME := $(shell uname)

# Used when building within docker
GOARCH := $(shell go env GOARCH)

# The location of the k8s YAML files
K8S_YAMLS_LOCATION_STAGING=./k8s/staging


# docker container registry
export CONTAINER_REGISTRY := gcr.io/flow-container-registry
export DOCKER_BUILDKIT := 1

# set `CRYPTO_FLAG` when building natively (not cross-compiling)
include crypto_adx_flag.mk

# needed for CI
.PHONY: noop
noop:
	@echo "This is a no-op target"

cmd/collection/collection:
	CGO_CFLAGS=$(CRYPTO_FLAG) go build -o cmd/collection/collection cmd/collection/main.go

cmd/util/util:
	CGO_CFLAGS=$(CRYPTO_FLAG) go build -o cmd/util/util cmd/util/main.go

.PHONY: update-core-contracts-version
update-core-contracts-version:
	# updates the core-contracts version in all of the go.mod files
	# usage example: CC_VERSION=0.16.0 make update-core-contracts-version
	./scripts/update-core-contracts.sh $(CC_VERSION)
	make tidy

.PHONY: update-cadence-version
update-cadence-version:
	# updates the cadence version in all of the go.mod files
	# usage example: CC_VERSION=0.16.0 make update-cadence-version
	./scripts/update-cadence.sh $(CC_VERSION)
	make tidy

.PHONY: unittest-main
unittest-main:
	# test all packages
	CGO_CFLAGS=$(CRYPTO_FLAG) go test $(if $(VERBOSE),-v,) -coverprofile=$(COVER_PROFILE) -covermode=atomic $(if $(RACE_DETECTOR),-race,) $(if $(JSON_OUTPUT),-json,) $(if $(NUM_RUNS),-count $(NUM_RUNS),) $(GO_TEST_PACKAGES)

.PHONY: install-mock-generators
install-mock-generators:
	cd ${GOPATH}; \
    go install github.com/vektra/mockery/v2@v2.21.4; \
    go install github.com/golang/mock/mockgen@v1.6.0;

.PHONY: install-tools
install-tools: check-go-version install-mock-generators
	cd ${GOPATH}; \
	go install github.com/golang/protobuf/protoc-gen-go@v1.3.2; \
	go install github.com/uber/prototool/cmd/prototool@v1.9.0; \
	go install github.com/gogo/protobuf/protoc-gen-gofast@latest; \
	go install golang.org/x/tools/cmd/stringer@master;

.PHONY: verify-mocks
verify-mocks: tidy generate-mocks
	git diff --exit-code

.SILENT: go-math-rand-check
go-math-rand-check:
	# check that the insecure math/rand Go package isn't used by production code.
	# `exclude` should only specify non production code (test, bench..).
	# If this check fails, try updating your code by using:
	#   - "crypto/rand" or "flow-go/utils/rand" for non-deterministic randomness
	#   - "onflow/crypto/random" for deterministic randomness
	grep --include=\*.go \
	--exclude=*test* --exclude=*helper* --exclude=*example* --exclude=*fixture* --exclude=*benchmark* --exclude=*profiler* \
    --exclude-dir=*test* --exclude-dir=*helper* --exclude-dir=*example* --exclude-dir=*fixture* --exclude-dir=*benchmark* --exclude-dir=*profiler* -rnw '"math/rand"'; \
    if [ $$? -ne 1 ]; then \
       echo "[Error] Go production code should not use math/rand package"; exit 1; \
    fi

.PHONY: code-sanity-check
code-sanity-check: go-math-rand-check

.PHONY: fuzz-fvm
fuzz-fvm:
	# run fuzz tests in the fvm package
	cd ./fvm && CGO_CFLAGS=$(CRYPTO_FLAG) go test -fuzz=Fuzz -run ^$$

.PHONY: test
test: verify-mocks unittest-main

.PHONY: integration-test
integration-test: docker-native-build-flow
	$(MAKE) -C integration integration-test

.PHONY: benchmark
benchmark: docker-native-build-flow
	$(MAKE) -C integration benchmark

.PHONY: coverage
coverage:
ifeq ($(COVER), true)
	# Cover summary has to produce cover.json
	COVER_PROFILE=$(COVER_PROFILE) ./cover-summary.sh
	# file has to be called index.html
	gocov-html cover.json > index.html
	# coverage.zip will automatically be picked up by teamcity
	zip coverage.zip index.html
endif

.PHONY: generate-openapi
generate-openapi:
	swagger-codegen generate -l go -i https://raw.githubusercontent.com/onflow/flow/master/openapi/access.yaml -D packageName=models,modelDocs=false,models -o engine/access/rest/models;
	go fmt ./engine/access/rest/models

.PHONY: generate
generate: generate-proto generate-mocks generate-fvm-env-wrappers

.PHONY: generate-proto
generate-proto:
	prototool generate protobuf

.PHONY: generate-fvm-env-wrappers
generate-fvm-env-wrappers:
	CGO_CFLAGS=$(CRYPTO_FLAG) go run ./fvm/environment/generate-wrappers fvm/environment/parse_restricted_checker.go

.PHONY: generate-mocks
generate-mocks: install-mock-generators
	mockery --name '(Connector|PingInfoProvider)' --dir=network/p2p --case=underscore --output="./network/mocknetwork" --outpkg="mocknetwork"
	CGO_CFLAGS=$(CRYPTO_FLAG) mockgen -destination=storage/mocks/storage.go -package=mocks github.com/onflow/flow-go/storage Blocks,Headers,Payloads,Collections,Commits,Events,ServiceEvents,TransactionResults
	CGO_CFLAGS=$(CRYPTO_FLAG) mockgen -destination=network/mocknetwork/mock_network.go -package=mocknetwork github.com/onflow/flow-go/network EngineRegistry
	mockery --name='.*' --dir=integration/benchmark/mocksiface --case=underscore --output="integration/benchmark/mock" --outpkg="mock"
	mockery --name=ExecutionDataStore --dir=module/executiondatasync/execution_data --case=underscore --output="./module/executiondatasync/execution_data/mock" --outpkg="mock"
	mockery --name=Downloader --dir=module/executiondatasync/execution_data --case=underscore --output="./module/executiondatasync/execution_data/mock" --outpkg="mock"
	mockery --name '(ExecutionDataRequester|IndexReporter)' --dir=module/state_synchronization --case=underscore --output="./module/state_synchronization/mock" --outpkg="state_synchronization"
	mockery --name 'ExecutionState' --dir=engine/execution/state --case=underscore --output="engine/execution/state/mock" --outpkg="mock"
	mockery --name 'BlockComputer' --dir=engine/execution/computation/computer --case=underscore --output="engine/execution/computation/computer/mock" --outpkg="mock"
	mockery --name 'ComputationManager' --dir=engine/execution/computation --case=underscore --output="engine/execution/computation/mock" --outpkg="mock"
	mockery --name '.*' --dir=engine/execution/computation/query --case=underscore --output="engine/execution/computation/query/mock" --outpkg="mock"
	mockery --name 'EpochComponentsFactory' --dir=engine/collection/epochmgr --case=underscore --output="engine/collection/epochmgr/mock" --outpkg="mock"
	mockery --name '.*' --dir=engine/execution/ --case=underscore --output="engine/execution/mock" --outpkg="mock"
	mockery --name 'Backend' --dir=engine/collection/rpc --case=underscore --output="engine/collection/rpc/mock" --outpkg="mock"
	mockery --name 'ProviderEngine' --dir=engine/execution/provider --case=underscore --output="engine/execution/provider/mock" --outpkg="mock"
	mockery --name '.*' --dir=state/cluster --case=underscore --output="state/cluster/mock" --outpkg="mock"
	mockery --name '.*' --dir=module --case=underscore --output="./module/mock" --outpkg="mock"
	mockery --name '.*' --dir=module/mempool --case=underscore --output="./module/mempool/mock" --outpkg="mempool"
	mockery --name '.*' --dir=module/component --case=underscore --output="./module/component/mock" --outpkg="component"
	mockery --name '.*' --dir=network --case=underscore --output="./network/mocknetwork" --outpkg="mocknetwork"
	mockery --name '.*' --dir=storage --case=underscore --output="./storage/mock" --outpkg="mock"
	mockery --name '.*' --dir="state/protocol" --case=underscore --output="state/protocol/mock" --outpkg="mock"
	mockery --name '.*' --dir="state/protocol/events" --case=underscore --output="./state/protocol/events/mock" --outpkg="mock"
	mockery --name '.*' --dir="state/protocol/protocol_state" --case=underscore --output="state/protocol/protocol_state/mock" --outpkg="mock"
	mockery --name '.*' --dir=engine/execution/computation/computer --case=underscore --output="./engine/execution/computation/computer/mock" --outpkg="mock"
	mockery --name '.*' --dir=engine/execution/state --case=underscore --output="./engine/execution/state/mock" --outpkg="mock"
	mockery --name '.*' --dir=engine/collection --case=underscore --output="./engine/collection/mock" --outpkg="mock"
	mockery --name 'complianceCore' --dir=engine/common/follower --exported --case=underscore --output="./engine/common/follower/mock" --outpkg="mock"
	mockery --name '.*' --dir=engine/common/follower/cache --case=underscore --output="./engine/common/follower/cache/mock" --outpkg="mock"
	mockery --name '.*' --dir=engine/consensus --case=underscore --output="./engine/consensus/mock" --outpkg="mock"
	mockery --name '.*' --dir=engine/consensus/approvals --case=underscore --output="./engine/consensus/approvals/mock" --outpkg="mock"
	rm -rf ./fvm/mock
	mockery --name '.*' --dir=fvm --case=underscore --output="./fvm/mock" --outpkg="mock"
	rm -rf ./fvm/environment/mock
	mockery --name '.*' --dir=fvm/environment --case=underscore --output="./fvm/environment/mock" --outpkg="mock"
	mockery --name '.*' --dir=ledger --case=underscore --output="./ledger/mock" --outpkg="mock"
	mockery --name 'ViolationsConsumer' --dir=network --case=underscore --output="./network/mocknetwork" --outpkg="mocknetwork"
	mockery --name '.*' --dir=network/p2p/ --case=underscore --output="./network/p2p/mock" --outpkg="mockp2p"
	mockery --name '.*' --dir=network/alsp --case=underscore --output="./network/alsp/mock" --outpkg="mockalsp"
	mockery --name 'Vertex' --dir="./module/forest" --case=underscore --output="./module/forest/mock" --outpkg="mock"
	mockery --name '.*' --dir="./consensus/hotstuff" --case=underscore --output="./consensus/hotstuff/mocks" --outpkg="mocks"
	mockery --name '.*' --dir="./engine/access/wrapper" --case=underscore --output="./engine/access/mock" --outpkg="mock"
	mockery --name 'API' --dir="./access" --case=underscore --output="./access/mock" --outpkg="mock"
	mockery --name 'API' --dir="./engine/protocol" --case=underscore --output="./engine/protocol/mock" --outpkg="mock"
	mockery --name '.*' --dir="./engine/access/state_stream" --case=underscore --output="./engine/access/state_stream/mock" --outpkg="mock"
	mockery --name 'BlockTracker' --dir="./engine/access/subscription" --case=underscore --output="./engine/access/subscription/mock"  --outpkg="mock"
	mockery --name 'ExecutionDataTracker' --dir="./engine/access/subscription" --case=underscore --output="./engine/access/subscription/mock"  --outpkg="mock"
	mockery --name 'ConnectionFactory' --dir="./engine/access/rpc/connection" --case=underscore --output="./engine/access/rpc/connection/mock" --outpkg="mock"
	mockery --name 'Communicator' --dir="./engine/access/rpc/backend" --case=underscore --output="./engine/access/rpc/backend/mock" --outpkg="mock"

	mockery --name '.*' --dir=model/fingerprint --case=underscore --output="./model/fingerprint/mock" --outpkg="mock"
	mockery --name 'ExecForkActor' --structname 'ExecForkActorMock' --dir=module/mempool/consensus/mock/ --case=underscore --output="./module/mempool/consensus/mock/" --outpkg="mock"
	mockery --name '.*' --dir=engine/verification/fetcher/ --case=underscore --output="./engine/verification/fetcher/mock" --outpkg="mockfetcher"
	mockery --name '.*' --dir=./cmd/util/ledger/reporters --case=underscore --output="./cmd/util/ledger/reporters/mock" --outpkg="mock"
	mockery --name 'Storage' --dir=module/executiondatasync/tracker --case=underscore --output="module/executiondatasync/tracker/mock" --outpkg="mocktracker"
	mockery --name 'ScriptExecutor' --dir=module/execution --case=underscore --output="module/execution/mock" --outpkg="mock"
	mockery --name 'StorageSnapshot' --dir=fvm/storage/snapshot --case=underscore --output="fvm/storage/snapshot/mock" --outpkg="mock"

	#temporarily make insecure/ a non-module to allow mockery to create mocks
	mv insecure/go.mod insecure/go2.mod
	if [ -f go.work ]; then mv go.work go2.work; fi
	mockery --name '.*' --dir=insecure/ --case=underscore --output="./insecure/mock"  --outpkg="mockinsecure"
	mv insecure/go2.mod insecure/go.mod
	if [ -f go2.work ]; then mv go2.work go.work; fi

# this ensures there is no unused dependency being added by accident
.PHONY: tidy
tidy:
	go mod tidy -v
	cd integration; go mod tidy -v
	cd crypto; go mod tidy -v
	cd cmd/testclient; go mod tidy -v
	cd insecure; go mod tidy -v
	git diff --exit-code

.PHONY: lint
lint: tidy
	# revive -config revive.toml -exclude storage/ledger/trie ./...
	golangci-lint run -v ./...

.PHONY: fix-lint
fix-lint:
	# revive -config revive.toml -exclude storage/ledger/trie ./...
	golangci-lint run -v --fix ./...

# Runs unit tests with different list of packages as passed by CI so they run in parallel
.PHONY: ci
ci: install-tools test

# Runs integration tests
.PHONY: ci-integration
ci-integration:
	$(MAKE) -C integration integration-test

# Runs benchmark tests
# NOTE: we do not need `docker-native-build-flow` as this is run as a separate step
# on Teamcity
.PHONY: ci-benchmark
ci-benchmark: install-tools
	$(MAKE) -C integration ci-benchmark

# Runs unit tests, test coverage, lint in Docker (for mac)
.PHONY: docker-ci
docker-ci:
	docker run --env RACE_DETECTOR=$(RACE_DETECTOR) --env COVER=$(COVER) --env JSON_OUTPUT=$(JSON_OUTPUT) \
		-v /run/host-services/ssh-auth.sock:/run/host-services/ssh-auth.sock -e SSH_AUTH_SOCK="/run/host-services/ssh-auth.sock" \
		-v "$(CURDIR)":/go/flow -v "/tmp/.cache":"/root/.cache" -v "/tmp/pkg":"/go/pkg" \
		-w "/go/flow" "$(CONTAINER_REGISTRY)/golang-cmake:v0.0.7" \
		make ci

# Runs integration tests in Docker  (for mac)
.PHONY: docker-ci-integration
docker-ci-integration:
	docker run \
		--env DOCKER_API_VERSION='1.39' \
		--network host \
		-v "$(CURDIR)":/go/flow -v "/tmp/.cache":"/root/.cache" -v "/tmp/pkg":"/go/pkg" \
		-v /tmp:/tmp \
		-v /var/run/docker.sock:/var/run/docker.sock \
		-v /run/host-services/ssh-auth.sock:/run/host-services/ssh-auth.sock -e SSH_AUTH_SOCK="/run/host-services/ssh-auth.sock" \
		-w "/go/flow" "$(CONTAINER_REGISTRY)/golang-cmake:v0.0.7" \
		make ci-integration

# only works on Debian
.SILENT: install-cross-build-tools
install-cross-build-tools:
	if [ "$(UNAME)" = "Debian" ] ; then \
		apt-get update && apt-get -y install apt-utils gcc-aarch64-linux-gnu ; \
	elif [ "$(UNAME)" = "Linux" ] ; then \
		apt-get update && apt-get -y install apt-utils gcc-aarch64-linux-gnu ; \
	else \
		echo "this target only works on Debian or Linux, host runs on" $(UNAME) ; \
	fi

.PHONY: docker-build
docker-build:
	docker build -f cmd/Dockerfile --target production \
		--label "git_tag=haswell_test" \
		-t "$(CONTAINER_REGISTRY)/consensus:haswell_test" .

.PHONY: docker-push
docker-push:
	docker push "$(CONTAINER_REGISTRY)/consensus:haswell_test"
	
PHONY: docker-build-util
docker-build-util:
	docker build -f cmd/Dockerfile --build-arg TARGET=./cmd/util --build-arg GOARCH=$(GOARCH) --build-arg VERSION=$(IMAGE_TAG) --build-arg CGO_FLAG=$(CRYPTO_FLAG) --target production \
		-t "$(CONTAINER_REGISTRY)/util:latest"  \
		-t "$(CONTAINER_REGISTRY)/util:$(IMAGE_TAG)" .

PHONY: tool-util
tool-util: docker-build-util
	docker container create --name util $(CONTAINER_REGISTRY)/util:latest;docker container cp util:/bin/app ./util;docker container rm util

PHONY: docker-build-remove-execution-fork
docker-build-remove-execution-fork:
	docker build -f cmd/Dockerfile --ssh default --build-arg TARGET=./cmd/util/cmd/remove-execution-fork --build-arg GOARCH=$(GOARCH) --build-arg VERSION=$(IMAGE_TAG) --build-arg CGO_FLAG=$(CRYPTO_FLAG) --target production \
		-t "$(CONTAINER_REGISTRY)/remove-execution-fork:latest" \
		-t "$(CONTAINER_REGISTRY)/remove-execution-fork:$(IMAGE_TAG)" .

PHONY: tool-remove-execution-fork
tool-remove-execution-fork: docker-build-remove-execution-fork
	docker container create --name remove-execution-fork $(CONTAINER_REGISTRY)/remove-execution-fork:latest;docker container cp remove-execution-fork:/bin/app ./remove-execution-fork;docker container rm remove-execution-fork

.PHONY: check-go-version
check-go-version:
	@bash -c '\
		MINGOVERSION=1.18; \
		function ver { printf "%d%03d%03d%03d" $$(echo "$$1" | tr . " "); }; \
		GOVER=$$(go version | sed -rne "s/.* go([0-9.]+).*/\1/p" ); \
		if [ "$$(ver $$GOVER)" -lt "$$(ver $$MINGOVERSION)" ]; then \
			echo "go $$GOVER is too old. flow-go only supports go $$MINGOVERSION and up."; \
			exit 1; \
		fi; \
		'

#----------------------------------------------------------------------
# CD COMMANDS
#----------------------------------------------------------------------

.PHONY: deploy-staging
deploy-staging: update-deployment-image-name-staging apply-staging-files monitor-rollout

# Staging YAMLs must have 'staging' in their name.
.PHONY: apply-staging-files
apply-staging-files:
	kconfig=$$(uuidgen); \
	echo "$$KUBECONFIG_STAGING" > $$kconfig; \
	files=$$(find ${K8S_YAMLS_LOCATION_STAGING} -type f \( --name "*.yml" -or --name "*.yaml" \)); \
	echo "$$files" | xargs -I {} kubectl --kubeconfig=$$kconfig apply -f {}

# Deployment YAMLs must have 'deployment' in their name.
.PHONY: update-deployment-image-name-staging
update-deployment-image-name-staging: CONTAINER=flow-test-net
update-deployment-image-name-staging:
	@files=$$(find ${K8S_YAMLS_LOCATION_STAGING} -type f \( --name "*.yml" -or --name "*.yaml" \) | grep deployment); \
	for file in $$files; do \
		patched=`openssl rand -hex 8`; \
		node=`echo "$$file" | grep -oP 'flow-\K\w+(?=-node-deployment.yml)'`; \
		kubectl patch -f $$file -p '{"spec":{"template":{"spec":{"containers":[{"name":"${CONTAINER}","image":"$(CONTAINER_REGISTRY)/'"$$node"':${IMAGE_TAG}"}]}}}}`' --local -o yaml > $$patched; \
		mv -f $$patched $$file; \
	done

.PHONY: monitor-rollout
monitor-rollout:
	kconfig=$$(uuidgen); \
	echo "$$KUBECONFIG_STAGING" > $$kconfig; \
	kubectl --kubeconfig=$$kconfig rollout status statefulsets.apps flow-collection-node-v1; \
	kubectl --kubeconfig=$$kconfig rollout status statefulsets.apps flow-consensus-node-v1; \
	kubectl --kubeconfig=$$kconfig rollout status statefulsets.apps flow-execution-node-v1; \
	kubectl --kubeconfig=$$kconfig rollout status statefulsets.apps flow-verification-node-v1
