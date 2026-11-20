package api

import (
	manager "github.com/Dhiraj1729/TargetServiceUpdate/internal/kubernetes"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHTTP(t *testing.T) {
	c := fake.NewClientset(&corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "app-0", Namespace: "test", Labels: map[string]string{manager.SelectorKey: "app-0"}}, Status: corev1.PodStatus{Conditions: []corev1.PodCondition{{Type: corev1.PodReady, Status: corev1.ConditionTrue}}}})
	h := New(&manager.Manager{Client: c.CoreV1(), Namespace: "test"})
	body := `{"serviceName":"active","podName":"app-0","ports":[{"name":"http","port":8080,"targetPort":8080}]}`
	for _, tt := range []struct {
		method, path, body string
		status             int
	}{
		{"GET", "/healthz", "", 200}, {"GET", "/readyz", "", 200},
		{"POST", "/api/v1/services", "{", 400},
		{"POST", "/api/v1/services", `{"unexpected":true}`, 400},
		{"POST", "/api/v1/services", body + ` {}`, 400},
		{"POST", "/api/v1/services", `{"podName":"` + strings.Repeat("x", 17000) + `"}`, 413},
		{"POST", "/api/v1/services", body + strings.Repeat(" ", 17000), 413},
		{"POST", "/api/v1/services", body, 201},
		{"POST", "/api/v1/services", body, 409},
		{"PUT", "/api/v1/services/active/target", `{"podName":"app-0"}`, 204},
		{"PUT", "/api/v1/services/absent/target", `{"podName":"app-0"}`, 404},
		{"DELETE", "/api/v1/services/active", "", 204},
		{"DELETE", "/api/v1/services/active", "", 204},
	} {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest(tt.method, tt.path, strings.NewReader(tt.body)))
		if w.Code != tt.status {
			t.Fatalf("%s %s got %d: %s", tt.method, tt.path, w.Code, w.Body.String())
		}
	}
}
