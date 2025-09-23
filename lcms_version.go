// lcms_version.go (in package golcms)
package golcms

import "fmt"

const (
	// Matches LCMS_VERSION macro (e.g., 2150 for 2.15)
	LCMSUpstreamEncoded = 2150

	LCMSUpstreamMajor = 2
	LCMSUpstreamMinor = 15
	LCMSUpstreamPatch = 0
)

func UpstreamVersion() string {
	return fmt.Sprintf("%d.%d.%d", LCMSUpstreamMajor, LCMSUpstreamMinor, LCMSUpstreamPatch)
}
