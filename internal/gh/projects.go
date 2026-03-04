package gh

// ProjectsV1Support provides type safety and readability around whether or not Projects v1 is supported
// by the targeted host.
//
// It is a sealed type to ensure that consumers must use the exported ProjectsV1Supported and ProjectsV1Unsupported
// variables to get an instance of the type.
type ProjectsV1Support interface {
	sealed()
}

type projectsV1Supported struct{}

func (projectsV1Supported) sealed() {}

type projectsV1Unsupported struct{}

func (projectsV1Unsupported) sealed() {}

var (
	// ProjectsV1Supported indicates the host supports classic projects.
	ProjectsV1Supported ProjectsV1Support = projectsV1Supported{}
	// ProjectsV1Unsupported indicates the host does not support classic projects.
	ProjectsV1Unsupported ProjectsV1Support = projectsV1Unsupported{}
)
