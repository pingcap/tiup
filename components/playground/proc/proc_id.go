package proc

import (
	"strings"

	"github.com/go4org/hashtriemap"
)

// ServiceID is a logical service identifier in playground.
//
// A ServiceID represents one start behavior (args/config/ports/ready strategy),
// and is what playground orchestration should use for planning, dependency and
// "critical" semantics.
//
// ServiceID is mode-independent. Mode only affects how a ServiceID maps to a
// RepoComponentID and its start implementation.
type ServiceID string

func (id ServiceID) String() string { return string(id) }

// RepoComponentID is a TiUP repository component ID.
//
// It is used for version resolution, downloading and locating binaries.
type RepoComponentID string

func (id RepoComponentID) String() string { return string(id) }

var componentDisplayNames hashtriemap.HashTrieMap[RepoComponentID, string]
var serviceDisplayNames hashtriemap.HashTrieMap[ServiceID, string]

// RegisterComponentDisplayName registers a user-facing name for a repository
// component ID.
//
// It is intended to be called from the init() function of each component
// implementation, so the naming rules stay close to the component itself.
func RegisterComponentDisplayName(componentID RepoComponentID, displayName string) {
	if componentID == "" || displayName == "" {
		return
	}
	componentDisplayNames.Store(componentID, displayName)
}

// ComponentDisplayName returns a user-facing name for a repository component ID.
//
// If no display name is registered, it falls back to a best-effort title-cased
// version of the id (split by '-' or '_').
func ComponentDisplayName(componentID RepoComponentID) string {
	if componentID == "" {
		return ""
	}
	if s, ok := componentDisplayNames.Load(componentID); ok && s != "" {
		return s
	}
	return titleCaseComponentID(componentID.String())
}

// RegisterServiceDisplayName registers a user-facing name for a service.
//
// It is intended to be called from the init() function of each service
// implementation, so the naming rules stay close to the component itself.
func RegisterServiceDisplayName(serviceID ServiceID, displayName string) {
	if serviceID == "" || displayName == "" {
		return
	}
	serviceDisplayNames.Store(serviceID, displayName)
}

// ServiceDisplayName returns a user-facing name for a service.
//
// If no service-specific display name is registered, it falls back to a
// best-effort title-cased form of the service ID.
func ServiceDisplayName(serviceID ServiceID) string {
	if serviceID == "" {
		return ""
	}
	if s, ok := serviceDisplayNames.Load(serviceID); ok && s != "" {
		return s
	}
	return titleCaseComponentID(string(serviceID))
}

func titleCaseComponentID(id string) string {
	parts := strings.FieldsFunc(id, func(r rune) bool {
		return r == '-' || r == '_'
	})
	if len(parts) == 0 {
		return id
	}
	for i, p := range parts {
		parts[i] = capitalizeASCII(p)
	}
	return strings.Join(parts, " ")
}

func capitalizeASCII(s string) string {
	if s == "" {
		return s
	}
	b := []byte(s)
	if b[0] >= 'a' && b[0] <= 'z' {
		b[0] = b[0] - 'a' + 'A'
	}
	return string(b)
}
