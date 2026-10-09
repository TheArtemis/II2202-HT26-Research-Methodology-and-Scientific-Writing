# Local state for bootstrap (chicken-and-egg: this stack creates the remote backend).
# After apply, `make bootstrap` / scripts/bootstrap.sh writes terraform/fleet/backend.hcl from outputs.
