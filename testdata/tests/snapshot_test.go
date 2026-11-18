package tests

import (
	"os"
	"strings"
	"testing"
)

func TestGeneratedSnapshots(t *testing.T) {
	userBindFile := mustRead(t, fromRoot(t, "internal/pkg/render/entbind/user.go"))
	if strings.Contains(userBindFile, "TODO: add custom converter") {
		t.Fatalf("[entcrud] generated entbind/user.go still contains TODO placeholder conversion")
	}
	if strings.Contains(userBindFile, "Skipped: incompatible types") {
		t.Fatalf("[entcrud] generated entbind/user.go still contains skipped incompatible type marker")
	}
	if !strings.Contains(userBindFile, "func CreateUser(") {
		t.Fatalf("[entcrud] generated entbind/user.go missing CreateUser")
	}

	postBindFile := mustRead(t, fromRoot(t, "internal/pkg/render/entbind/post.go"))
	if !strings.Contains(postBindFile, "source.SetIPV4(entmap.ToEntPost_IPV4(target.IpV4))") {
		t.Fatalf("[entcrud] generated entbind/post.go missing SetIPV4 binding")
	}

	protoFile := mustRead(t, fromRoot(t, "proto/entpb/entpb.proto"))
	if !strings.Contains(protoFile, "repeated Post posts = 16;") {
		t.Fatalf("[entproto] generated proto missing user.posts edge")
	}
	if !strings.Contains(protoFile, "repeated Group groups = 17;") {
		t.Fatalf("[entproto] generated proto missing user.groups edge")
	}
	if !strings.Contains(protoFile, "User author = 9;") {
		t.Fatalf("[entproto] generated proto missing post.author edge")
	}
}

func mustRead(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(b)
}
