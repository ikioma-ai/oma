package brand

const (
	Name              = "oma"
	DisplayName       = "Oma"
	Wordmark          = "OMA"
	Repo              = "ikioma-ai/oma"
	GitHubReleasesURL = "https://api.github.com/repos/" + Repo + "/releases/latest"
	ServicePrefix     = "com.ikioma."

	EnvPrefix        = "OMA_"
	ReducedMotionEnv = EnvPrefix + "REDUCED_MOTION"
	NoSelfUpdateEnv  = EnvPrefix + "NO_SELF_UPDATE"
	InteractiveEnv   = EnvPrefix + "INTERACTIVE"
)
