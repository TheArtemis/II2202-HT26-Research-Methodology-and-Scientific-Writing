# AWS experiment fleet — operator entrypoints
# Run from infra/aws (or: make -C infra/aws <target>).
#
# Lifecycle:
#   make bootstrap
#   make infra-apply CAMPAIGN_ID=full-YYYYMMDD SSH_CIDR=<you>/32
#   make configure GIT_SHA=$(git rev-parse HEAD)
#   make qualify
#   make launch CONFIG=experiments/configs/full.yaml
#   make watch          # laptop: 5m S3 poll → merge → summarize → viz
#   make finish         # final merge + upload + destroy

SHELL := /bin/bash
.SHELLFLAGS := -eu -o pipefail -c

SCRIPTS := scripts
BASH := bash
export CAMPAIGN_ID SSH_CIDR SSH_PUBLIC_KEY GIT_SHA REPO_URL CONFIG RESULTS_BUCKET
export AWS_REGION WORKERS TOTAL_TRIALS WATCH_INTERVAL_SEC ANSIBLE_ARGS

.PHONY: help bootstrap infra-apply configure qualify launch status \
	collect merge watch finish infra-destroy apply

help:
	@echo "Targets: bootstrap infra-apply configure qualify launch status collect merge watch finish infra-destroy"
	@echo "Required for apply: CAMPAIGN_ID=… SSH_CIDR=<ip>/32"
	@echo "Required for configure: GIT_SHA=<commit>"
	@echo "Defaults: CONFIG=experiments/configs/full.yaml TOTAL_TRIALS=900 WORKERS=8 WATCH_INTERVAL_SEC=300"

bootstrap:
	@$(BASH) $(SCRIPTS)/bootstrap.sh

apply: infra-apply

infra-apply:
	@$(BASH) $(SCRIPTS)/infra-apply.sh

configure:
	@$(BASH) $(SCRIPTS)/configure.sh

qualify:
	@$(BASH) $(SCRIPTS)/qualify.sh

launch:
	@$(BASH) $(SCRIPTS)/launch.sh

status:
	@$(BASH) $(SCRIPTS)/status.sh

collect:
	@$(BASH) $(SCRIPTS)/collect.sh

merge:
	@$(BASH) $(SCRIPTS)/merge.sh

watch:
	@$(BASH) $(SCRIPTS)/watch.sh

finish:
	@$(BASH) $(SCRIPTS)/finish.sh

infra-destroy:
	@$(BASH) $(SCRIPTS)/infra-destroy.sh
