package vault

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/sys/windows"
)

func TestPrivateOutputACL(t *testing.T) {
	path := filepath.Join(t.TempDir(), "key")
	if err := GenerateKey(path); err != nil {
		t.Fatal(err)
	}
	sd, err := windows.GetNamedSecurityInfo(path, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION)
	if err != nil {
		t.Fatal(err)
	}
	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		t.Fatal(err)
	}
	text := sd.String()
	if !strings.Contains(text, "D:P") || !strings.Contains(text, user.User.Sid.String()) || strings.Contains(text, ";;;WD)") || strings.Contains(text, ";;;BU)") {
		t.Fatalf("unexpected access control: %s", text)
	}
	if _, err := os.ReadFile(path); err != nil {
		t.Fatal(err)
	}
}
