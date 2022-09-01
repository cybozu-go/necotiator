package constants

// MetaPrefix is the MetaPrefix for labels, annotations, and finalizers of necotiator.
const MetaPrefix = "necotiator.cybozu.io/"

// Finalizer is the finalizer ID of necotiator.
const Finalizer = MetaPrefix + "finalizer"

// Labels
const (
	LabelTenant    = MetaPrefix + "tenant"
	LabelCreatedBy = "app.kubernetes.io/created-by"
)

// Label or annotation values
const (
	CreatedBy                = "necotiator"
	ResourceQuotaNameDefault = "default"
)

// Event Recorder Name
const (
	EventRecorderName = "necotiator"
)

// Controller Name
const (
	ControllerName = "necotiator-controller"
)
