//go:build newman

package postgres_test

import (
	"context"
	"errors"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"firebase.google.com/go/v4/auth"
	"github.com/fahrypratama17/EcoSnap-BE/internal/bootstrap"
	"github.com/fahrypratama17/EcoSnap-BE/internal/config"
)

type newmanTokenVerifier struct{}

var newmanTokenUIDs = map[string]string{
	"ci-user":           "plain-user",
	"ci-mitra":          "owner-a",
	"ci-register-user":  "newman-register-user",
	"ci-register-mitra": "newman-register-mitra",
}

func (newmanTokenVerifier) VerifyIDToken(_ context.Context, token string) (*auth.Token, error) {
	uid, ok := newmanTokenUIDs[token]
	if !ok {
		return nil, errors.New("invalid synthetic token")
	}
	return &auth.Token{UID: uid, Claims: map[string]interface{}{"email": uid + "@example.test", "email_verified": true}}, nil
}

func (v newmanTokenVerifier) VerifyIDTokenAndCheckRevoked(ctx context.Context, token string) (*auth.Token, error) {
	return v.VerifyIDToken(ctx, token)
}

func TestNewmanFullFlow(t *testing.T) {
	if os.Getenv("ECOSNAP_TEST_ADMIN_URL") == "" {
		t.Fatal("set ECOSNAP_TEST_ADMIN_URL to a local disposable PostgreSQL cluster")
	}
	if _, err := exec.LookPath("node"); err != nil {
		t.Fatal("node is required for the Newman full-flow integration test")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	pool := accountTestDatabase(t)
	depositFixture(ctx, t, pool)
	if _, err := pool.Exec(ctx, `INSERT INTO public.rewards(id,name,cost_points,stock,is_active,is_demo) VALUES
	 ('80000000-0000-4000-8000-0000000000b1','CI reward',500,20,true,true),
	 ('80000000-0000-4000-8000-0000000000b2','Expensive CI reward',100000,20,true,true)`); err != nil {
		t.Fatal("seed Newman rewards:", err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO public.articles(id,title,summary,category,reading_minutes,is_published,published_at) VALUES
	 ('91000000-0000-4000-8000-000000000001','Newman article 1','CI','NEWS',1,true,now()-interval '1 minute'),
	 ('91000000-0000-4000-8000-000000000002','Newman article 2','CI','NEWS',1,true,now()-interval '2 minutes'),
	 ('91000000-0000-4000-8000-000000000003','Newman article 3','CI','NEWS',1,true,now()-interval '3 minutes'),
	 ('91000000-0000-4000-8000-000000000004','Newman article 4','CI','NEWS',1,true,now()-interval '4 minutes'),
	 ('91000000-0000-4000-8000-000000000005','Newman article 5','CI','NEWS',1,true,now()-interval '5 minutes')`); err != nil {
		t.Fatal("seed Newman articles:", err)
	}

	srv := httptest.NewServer(bootstrap.NewServer(config.Config{}, newmanTokenVerifier{}, pool).Handler)
	defer srv.Close()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot locate Newman test source")
	}
	root := filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", ".."))
	script := filepath.Join(root, "scripts", "newman", "run-full-flow.cjs")
	cmd := exec.CommandContext(ctx, "node", script, "--base-url", srv.URL, "--root", root)
	cmd.Dir = root
	output, err := cmd.CombinedOutput()
	if len(output) > 0 {
		t.Logf("Newman summary: %s", output)
	}
	if err != nil {
		t.Fatalf("Newman full-flow failed: %v", err)
	}
}
