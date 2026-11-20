# Environment verification

Infrastructure and application deployment are separate: only
`scripts/hsm-deploy.sh` applies application bootstrap and manager manifests.
`scripts/deploy.sh` remains a compatibility wrapper.

1. `make shell-check`: shell syntax validation.
2. `make tools doctor`: reuse installed commands and inspect versions/capacity.
3. `make deps`: resolve the pinned official Kubernetes Go modules.
4. `make cluster-up`: two Ready nodes on v1.37.0, CoreDNS and storage provisioner
   available. No application resources created or applied.
5. `make cluster-verify`: infrastructure checks pass even without hsm-system.
6. Repeat `make cluster-up`: reuse infrastructure without applying bootstrap.
7. `make cluster-down`, then `make cluster-up`: verify preserved-state restart.
8. After startup, plain `kubectl get nodes` and `kubectl get pods -A` work
   without exports. Context namespace is default. Other kubeconfig entries
   survive activation; unrelated entries named hsm must be rejected.
9. `make image deploy`: create namespace/service account/Role/RoleBinding and
   HSM Deployment/Service; switch context namespace to hsm-system.
10. `make hsm-verify`: application rollout is Ready; Service CRUD/patch and pod
    get allowed in hsm-system; pod deletion and cross-namespace writes denied.

Evidence is written to `work/setup/environment.txt`. These checks prove the
development environment. `make demo smoke` tests application routing using two
StatefulSet pods, REST calls through a localhost port-forward, EndpointSlice
convergence and real HTTP traffic through the ClusterIP. `make test` covers
create, duplicate create, selector switching with IP/ports/metadata preserved,
delete, invalid input, target readiness/labels, unmanaged Services and conflicts.

If startup is interrupted, keep the ownership marker and rerun cluster-up.
Version mismatches or unexpected profile/context identities cause refusal.
No destructive cleanup command is included in this setup milestone.

## Results — 20 November 2026

Passed shell syntax checks and `go mod verify`. All seven prerequisite CLIs
were already installed and reused. Fresh provisioning produced two Ready arm64
nodes on Kubernetes v1.37.0 with containerd 2.3.4, available CoreDNS and the
expected namespace permissions. Repeated startup left bootstrap objects
unchanged. Stopping both nodes and Colima, then restarting, preserved node and
namespace identity and passed the same readiness/RBAC checks.

The lifecycle test exposed Minikube's removal of kubeconfig entries on stop;
startup now accepts an empty project kubeconfig and lets Minikube recreate it.
Unexpected nonempty context identities still fail validation. The environment
was left running after successful verification.

## Application results — 20 November 2026

`make test` passed Go unit/HTTP tests, go vet and shell syntax checks. Native and
Linux/arm64 builds succeeded. The manager Deployment passed its readiness probe
under the namespace-scoped ServiceAccount. Two demo pods became Ready on separate
nodes. Live smoke evidence: `work/smoke/run.nOohtW/result.txt`.

Verified using curl from the host through a loopback port-forward:

- POST returned 201 and traffic through the created ClusterIP reached hsm-demo-0.
- PUT returned 204; EndpointSlice and traffic switched to hsm-demo-1.
- The Service retained its ClusterIP across the update.
- DELETE returned 204 and Kubernetes confirmed the Service was absent.
- Repeated DELETE returned 204.
- DELETE of the unmanaged manager API Service returned 403.

The smoke Service was deleted; the manager Deployment and demo StatefulSet remain
running for manual testing. This verifies requested Service routing changes, not
automatic active/standby election or application-state replication.

## Infrastructure/application separation verification

Shell syntax checks and Make target resolution passed after separation. The
revised `make cluster-up` completed successfully: both nodes, CoreDNS and the
storage provisioner became Ready, with no application manifests applied or
application RBAC checks performed. Normal kubectl selected hsm with namespace
default. Existing hsm-system resources were retained; startup did not redeploy
them. Application deployment is now explicit via `make deploy` / `make hsm-deploy`.
