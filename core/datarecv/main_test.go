package datarecv

import "testing"

// A source is taken for a fetch only when the install fetches it: a text the
// install reads from the local filesystem is a local source, whatever prefix
// it wears.
func TestTextHasLocalSource(t *testing.T) {
	for s, want := range map[string]bool{
		"/init/x from ./cfg/{name} key x source https://example.com/app.conf":         false,
		"/init/x from ./cfg/{name} key x source http://example.com/app.conf template": false,
		"/init/x from ./cfg/{name} key x source /etc/app.conf":                        true,
		"/init/x from ./cfg/{name} key x source http:///etc/shadow":                   true,
		"/init/x from ./cfg/{name} key x source https:/etc/shadow":                    true,
		"/init/x from ./cfg/{name} key x source http:etc/shadow":                      true,
		"/init/x from ./cfg/{name} key x mode 0644":                                   false,
		"/etc/ mode 0750\n/init/y from ./cfg/{name} key y source http:///etc/shadow":  true,
	} {
		if got := TextHasLocalSource(s); got != want {
			t.Errorf("%q: got %v, want %v", s, got, want)
		}
	}
}
