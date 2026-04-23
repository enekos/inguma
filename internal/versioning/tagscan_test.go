package versioning

import (
	"reflect"
	"testing"
)

func TestScanTags(t *testing.T) {
	in := []string{"v1.0.0", "v1.2.3", "release-2", "v0.1.0-beta.1", "main", "v2"}
	got := ScanTags(in)
	want := []string{"v0.1.0-beta.1", "v1.0.0", "v1.2.3"}
	gotStr := make([]string, len(got))
	for i, v := range got {
		gotStr[i] = v.Canonical()
	}
	if !reflect.DeepEqual(gotStr, want) {
		t.Fatalf("got %v want %v", gotStr, want)
	}
}
