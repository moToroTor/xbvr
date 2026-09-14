package dms

import (
	"strings"
	"testing"
)

// xbapps/xbvr#725: <upnp:class> must always be a valid object.item.* class,
// never object.item.applicationItem.
func TestUpnpClassForMimeType(t *testing.T) {
	cases := map[mimeType]string{
		"video/mp4":                        "object.item.videoItem",
		"video/avi":                        "object.item.videoItem",
		"application/vnd.rn-realmedia-vbr": "object.item.videoItem", // .rmvb: video, but Type() is "application"
		"application/octet-stream":         "object.item.videoItem", // unknown: safe fallback, never applicationItem
		"audio/mpeg":                       "object.item.audioItem",
		"image/jpeg":                       "object.item.imageItem",
	}
	for mt, want := range cases {
		if got := upnpClassForMimeType(mt); got != want {
			t.Errorf("upnpClassForMimeType(%q) = %q, want %q", mt, got, want)
		}
		if !strings.HasPrefix(upnpClassForMimeType(mt), "object.item.") ||
			strings.Contains(upnpClassForMimeType(mt), "application") {
			t.Errorf("upnpClassForMimeType(%q) = %q, not schema-valid", mt, upnpClassForMimeType(mt))
		}
	}
}
