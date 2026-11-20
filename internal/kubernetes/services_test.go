package kubernetes

import (
	"context"
	"errors"
	"reflect"
	"testing"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes/fake"
	ktesting "k8s.io/client-go/testing"
)

func pod(name string) *corev1.Pod {
	return &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "test", Labels: map[string]string{SelectorKey: name}}, Status: corev1.PodStatus{Conditions: []corev1.PodCondition{{Type: corev1.PodReady, Status: corev1.ConditionTrue}}}}
}
func request() CreateRequest { return CreateRequest{"active", "app-0", []Port{{"http", 8080, 8080}}} }
func TestLifecycle(t *testing.T) {
	ctx := context.Background()
	c := fake.NewClientset(pod("app-0"), pod("app-1"))
	m := Manager{c.CoreV1(), "test"}
	svc, err := m.Create(ctx, request())
	if err != nil {
		t.Fatal(err)
	}
	if svc.Spec.Selector[SelectorKey] != "app-0" || svc.Labels[ManagedBy] != ManagerName {
		t.Fatal("incorrect created service", svc)
	}
	if _, err = m.Create(ctx, request()); !apierrors.IsAlreadyExists(err) {
		t.Fatalf("duplicate: %v", err)
	}
	svc.Spec.ClusterIP = "10.0.0.42"
	svc.Spec.Selector["stale"] = "label"
	svc.Annotations = map[string]string{"keep": "me"}
	_, err = c.CoreV1().Services("test").Update(ctx, svc, metav1.UpdateOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if err = m.UpdateTarget(ctx, "active", "app-1"); err != nil {
		t.Fatal(err)
	}
	updated, _ := c.CoreV1().Services("test").Get(ctx, "active", metav1.GetOptions{})
	if !reflect.DeepEqual(updated.Spec.Selector, map[string]string{SelectorKey: "app-1"}) || updated.Spec.ClusterIP != svc.Spec.ClusterIP || !reflect.DeepEqual(updated.Spec.Ports, svc.Spec.Ports) || !reflect.DeepEqual(updated.Annotations, svc.Annotations) {
		t.Fatal("selector patch changed unrelated fields", updated)
	}
	for i := 0; i < 2; i++ {
		if err := m.Delete(ctx, "active"); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := c.CoreV1().Pods("test").Get(ctx, "app-1", metav1.GetOptions{}); err != nil {
		t.Fatal("pod was affected", err)
	}
}

func TestRejections(t *testing.T) {
	for _, test := range []struct {
		name   string
		modify func(*corev1.Pod, *CreateRequest)
		code   int
	}{
		{"bad service", func(p *corev1.Pod, r *CreateRequest) { r.ServiceName = "Bad_Name" }, 400},
		{"bad port", func(p *corev1.Pod, r *CreateRequest) { r.Ports[0].Port = 0 }, 400},
		{"not ready", func(p *corev1.Pod, r *CreateRequest) { p.Status.Conditions = nil }, 422},
		{"missing label", func(p *corev1.Pod, r *CreateRequest) { p.Labels = nil }, 422},
		{"terminating", func(p *corev1.Pod, r *CreateRequest) { now := metav1.Now(); p.DeletionTimestamp = &now }, 422},
	} {
		t.Run(test.name, func(t *testing.T) {
			p, r := pod("app-0"), request()
			test.modify(p, &r)
			c := fake.NewClientset(p)
			m := Manager{c.CoreV1(), "test"}
			_, err := m.Create(context.Background(), r)
			var rejected *Error
			if !errors.As(err, &rejected) || rejected.Code != test.code {
				t.Fatalf("got %v", err)
			}
			list, _ := c.CoreV1().Services("test").List(context.Background(), metav1.ListOptions{})
			if len(list.Items) != 0 {
				t.Fatal("created rejected Service")
			}
		})
	}
}

func TestOwnershipAndErrors(t *testing.T) {
	ctx := context.Background()
	c := fake.NewClientset(pod("app-0"), &corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: "foreign", Namespace: "test"}})
	m := Manager{c.CoreV1(), "test"}
	for _, err := range []error{m.UpdateTarget(ctx, "foreign", "app-0"), m.Delete(ctx, "foreign")} {
		var rejected *Error
		if !errors.As(err, &rejected) || rejected.Code != 403 {
			t.Fatalf("ownership: %v", err)
		}
	}
	if err := m.UpdateTarget(ctx, "missing", "app-0"); !apierrors.IsNotFound(err) {
		t.Fatal(err)
	}
	r := request()
	r.PodName = "missing"
	if _, err := m.Create(ctx, r); !apierrors.IsNotFound(err) {
		t.Fatal(err)
	}
	if _, err := m.Create(ctx, request()); err != nil {
		t.Fatal(err)
	}
	c.PrependReactor("patch", "services", func(action ktesting.Action) (bool, runtime.Object, error) {
		return true, nil, apierrors.NewConflict(schema.GroupResource{Resource: "services"}, "active", errors.New("concurrent write"))
	})
	if err := m.UpdateTarget(ctx, "active", "app-0"); !apierrors.IsConflict(err) {
		t.Fatalf("conflict swallowed: %v", err)
	}
}
