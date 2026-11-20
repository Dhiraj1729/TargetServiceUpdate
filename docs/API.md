# REST API and manual test

Run `make image deploy demo`, then keep `make port-forward` running in a terminal.
Use another terminal for these commands. All resources are in `hsm-system`.

```sh
# Create: 201, with serviceName, namespace, clusterIP, podName and ports.
curl -i -X POST http://localhost:8080/api/v1/services \
  -H 'Content-Type: application/json' \
  -d '{"serviceName":"application-active","podName":"hsm-demo-0","ports":[{"name":"http","port":8080,"targetPort":8080}]}'

# Check real traffic through the Service: response should be hsm-demo-0.
kubectl exec hsm-demo-0 -- wget -qO- http://application-active:8080

# Switch: 204, preserving ClusterIP and ports.
curl -i -X PUT http://localhost:8080/api/v1/services/application-active/target \
  -H 'Content-Type: application/json' -d '{"podName":"hsm-demo-1"}'

# Response should become hsm-demo-1 after network convergence.
kubectl exec hsm-demo-0 -- wget -qO- http://application-active:8080

# Delete just the Service: 204, even if it is already absent.
curl -i -X DELETE http://localhost:8080/api/v1/services/application-active
```

Kubernetes updates EndpointSlices and network rules asynchronously. Retry traffic
checks briefly after a selector change; existing TCP connections are not migrated.
The automated `make smoke` waits for both EndpointSlice convergence and real
HTTP traffic to the expected pod. It verifies stable ClusterIP, repeat delete,
and rejection of deletion of the manager's own API Service.

## Contract

| Endpoint | Success | Input |
| --- | --- | --- |
| POST `/api/v1/services` | 201 | `serviceName`, `podName`, nonempty `ports` |
| PUT `/api/v1/services/{serviceName}/target` | 204 | `podName` |
| DELETE `/api/v1/services/{serviceName}` | 204 | Service name in path |
| GET `/healthz` | 200 | Process health |
| GET `/readyz` | 200 or 503 | Kubernetes Service-list access |

Ports are numeric TCP ports with unique names and service port numbers. Unknown
JSON fields, multiple objects and bodies over 16 KiB are rejected. Errors return
JSON with an `error` string: 400 invalid input, 403 unmanaged Service, 413 oversized
body, 404 absent
Service/target pod, 409 duplicate create/concurrent change, 422 target not Ready,
terminating or missing its label, 503 Kubernetes access unavailable, or 500
unexpected failure. PUT requires an existing Service. POST on an existing name
does not modify it.

The target must be in the manager's namespace and have
`statefulset.kubernetes.io/pod-name=<podName>`. StatefulSets provide this label.
Create marks Services with
`app.kubernetes.io/managed-by=service-target-manager`; update/delete require it.
To enroll an existing non-headless ClusterIP Service, deliberately add that label
after checking the resource. Updating replaces the entire selector; other
settings, including ClusterIP, ports and unrelated metadata, are preserved.
The manager's API Service is not enrolled and cannot be deleted via the API.

The API assumes trusted callers on the local/internal network and has no caller
authentication. Kubernetes RBAC restricts the manager to `hsm-system`. Automatic
failure detection, pod promotion, distributed election and stale-request fencing
are outside this implementation. Resource-version preconditions reject
concurrent changes; sequential calls still apply in received order. Callers must
serialize activation decisions and handle conflicts. Pod readiness validation
is a point-in-time check and cannot guarantee the pod remains healthy.

For local debugging, `make build && ./bin/manager` uses your active kubeconfig.
Inside Kubernetes, the manager uses its ServiceAccount. `HSM_NAMESPACE` defaults
to `hsm-system`, and `HSM_LISTEN_ADDR` defaults to `:8080`.
