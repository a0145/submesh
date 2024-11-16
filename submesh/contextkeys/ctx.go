package contextkeys

type ContextKey string

const (
	JSONFileLogger ContextKey = "JSONFileLogger"
	RAWFileLogger  ContextKey = "RAWFileLogger"
	Logger         ContextKey = "logger"
	State          ContextKey = "state"
	Config         ContextKey = "config"
	AtomicLevel    ContextKey = "atomicLevel"
	AppVersion     ContextKey = "AppVersion"
)
